package gemini

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/specvital/worker/internal/domain/specview"
)

const (
	// taxonomyChunkSize is the maximum number of files per taxonomy extraction call.
	// 300 files targets ~12K output tokens (18% of 65K limit) for larger safety margin.
	// Reduced from 400 to prevent MAX_TOKENS truncation on complex codebases.
	taxonomyChunkSize = 300

	// taxonomyWaveSize limits concurrent taxonomy extraction calls.
	// Lower than Stage 2 (waveSize=10) as Stage 1 is heavier.
	taxonomyWaveSize = 3
)

// TaxonomyChunk represents a subset of files for chunked taxonomy extraction.
type TaxonomyChunk struct {
	ChunkIndex   int
	Files        []specview.TaxonomyFileInfo
	GlobalOffset int // Starting index in original file list
	TotalChunks  int
	TotalFiles   int
}

// ChunkedTaxonomyResult holds the result of a single chunk's taxonomy extraction.
type ChunkedTaxonomyResult struct {
	ChunkIndex   int
	FileCount    int // Actual number of files in this chunk
	GlobalOffset int
	Taxonomy     *specview.TaxonomyOutput
	Usage        *specview.TokenUsage
	Err          error
}

// extractTaxonomyWithChunking handles large file sets by splitting into chunks.
// Returns unified taxonomy by merging chunk results.
func (p *Provider) extractTaxonomyWithChunking(ctx context.Context, input specview.TaxonomyInput) (*specview.TaxonomyOutput, *specview.TokenUsage, error) {
	if len(input.Files) == 0 {
		return nil, nil, fmt.Errorf("%w: no files for taxonomy extraction", specview.ErrInvalidInput)
	}

	// Small input: use direct extraction
	if len(input.Files) <= taxonomyChunkSize {
		return p.extractTaxonomy(ctx, input)
	}

	chunks := splitTaxonomyInput(input, taxonomyChunkSize)
	slog.InfoContext(ctx, "splitting taxonomy extraction into chunks",
		"total_files", len(input.Files),
		"chunk_count", len(chunks),
		"chunk_size", taxonomyChunkSize,
	)

	results, err := p.processTaxonomyChunks(ctx, chunks, input.Language)
	if err != nil {
		return nil, nil, err
	}

	taxonomy, totalUsage := mergeTaxonomyChunks(ctx, results, len(input.Files))

	slog.InfoContext(ctx, "chunked taxonomy extraction complete",
		"domain_count", len(taxonomy.Domains),
		"total_prompt_tokens", totalUsage.PromptTokens,
		"total_output_tokens", totalUsage.CandidatesTokens,
	)

	return taxonomy, totalUsage, nil
}

// splitTaxonomyInput divides files into processable chunks.
func splitTaxonomyInput(input specview.TaxonomyInput, chunkSize int) []TaxonomyChunk {
	totalFiles := len(input.Files)
	chunkCount := (totalFiles + chunkSize - 1) / chunkSize
	chunks := make([]TaxonomyChunk, 0, chunkCount)

	for i := 0; i < totalFiles; i += chunkSize {
		end := min(i+chunkSize, totalFiles)
		chunkFiles := make([]specview.TaxonomyFileInfo, end-i)

		// Remap indices to chunk-local (0-based)
		for j, file := range input.Files[i:end] {
			chunkFiles[j] = specview.TaxonomyFileInfo{
				DomainHints: file.DomainHints,
				Index:       j, // Local index within chunk
				Path:        file.Path,
				TestCount:   file.TestCount,
			}
		}

		chunks = append(chunks, TaxonomyChunk{
			ChunkIndex:   len(chunks),
			Files:        chunkFiles,
			GlobalOffset: i,
			TotalChunks:  chunkCount,
			TotalFiles:   totalFiles,
		})
	}

	return chunks
}

// processTaxonomyChunks processes all chunks in waves.
func (p *Provider) processTaxonomyChunks(ctx context.Context, chunks []TaxonomyChunk, lang specview.Language) ([]ChunkedTaxonomyResult, error) {
	results := make([]ChunkedTaxonomyResult, 0, len(chunks))

	for waveStart := 0; waveStart < len(chunks); waveStart += taxonomyWaveSize {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("taxonomy chunking cancelled: %w", err)
		}

		waveEnd := min(waveStart+taxonomyWaveSize, len(chunks))
		waveChunks := chunks[waveStart:waveEnd]

		slog.InfoContext(ctx, "processing taxonomy wave",
			"wave_start", waveStart,
			"wave_end", waveEnd,
			"wave_size", len(waveChunks),
		)

		waveResults := p.processTaxonomyWave(ctx, waveChunks, lang)
		results = append(results, waveResults...)

		// Check for fatal errors (all chunks failed)
		allFailed := true
		for _, r := range waveResults {
			if r.Err == nil {
				allFailed = false
				break
			}
		}
		if allFailed {
			return nil, fmt.Errorf("all chunks in wave failed: %w", waveResults[0].Err)
		}
	}

	return results, nil
}

// processTaxonomyWave processes a single wave of chunks concurrently.
func (p *Provider) processTaxonomyWave(ctx context.Context, chunks []TaxonomyChunk, lang specview.Language) []ChunkedTaxonomyResult {
	resultCh := make(chan ChunkedTaxonomyResult, len(chunks))
	var wg sync.WaitGroup

	for _, chunk := range chunks {
		wg.Add(1)
		go func(c TaxonomyChunk) {
			defer wg.Done()

			chunkInput := specview.TaxonomyInput{
				AnalysisID: fmt.Sprintf("chunk_%d", c.ChunkIndex),
				Files:      c.Files,
				Language:   lang,
			}

			taxonomy, usage, err := p.extractTaxonomy(ctx, chunkInput)

			resultCh <- ChunkedTaxonomyResult{
				ChunkIndex:   c.ChunkIndex,
				FileCount:    len(c.Files),
				GlobalOffset: c.GlobalOffset,
				Taxonomy:     taxonomy,
				Usage:        usage,
				Err:          err,
			}
		}(chunk)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]ChunkedTaxonomyResult, 0, len(chunks))
	for r := range resultCh {
		results = append(results, r)
	}

	// Sort by chunk index for deterministic merging
	sort.Slice(results, func(i, j int) bool {
		return results[i].ChunkIndex < results[j].ChunkIndex
	})

	return results
}

// mergeTaxonomyChunks combines chunk results into unified taxonomy.
// Handles partial failures by generating heuristic taxonomy for failed chunks.
func mergeTaxonomyChunks(ctx context.Context, results []ChunkedTaxonomyResult, totalFiles int) (*specview.TaxonomyOutput, *specview.TokenUsage) {
	totalUsage := &specview.TokenUsage{}
	domainMap := make(map[string]*specview.TaxonomyDomain)

	for _, result := range results {
		if result.Usage != nil {
			aggregateUsage(totalUsage, result.Usage)
		}

		var taxonomy *specview.TaxonomyOutput
		if result.Err != nil || result.Taxonomy == nil {
			// Generate heuristic for failed chunk
			slog.WarnContext(ctx, "chunk taxonomy extraction failed, using heuristic",
				"chunk_index", result.ChunkIndex,
				"error", result.Err,
			)
			taxonomy = generateHeuristicTaxonomyForChunk(result)
		} else {
			taxonomy = result.Taxonomy
		}

		// Merge domains with global index remapping
		for _, domain := range taxonomy.Domains {
			normalizedName := normalizeDomainName(domain.Name)

			if existing, ok := domainMap[normalizedName]; ok {
				mergeFeatures(existing, domain, result.GlobalOffset)
			} else {
				domainMap[normalizedName] = remapDomainIndices(domain, result.GlobalOffset)
			}
		}
	}

	// Convert map to sorted slice
	domains := make([]specview.TaxonomyDomain, 0, len(domainMap))
	for _, domain := range domainMap {
		domains = append(domains, *domain)
	}
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Name < domains[j].Name
	})

	// Ensure Uncategorized exists if empty
	if len(domains) == 0 {
		allIndices := make([]int, totalFiles)
		for i := range allIndices {
			allIndices[i] = i
		}
		domains = []specview.TaxonomyDomain{
			{
				Description: "All test files",
				Features: []specview.TaxonomyFeature{
					{
						FileIndices: allIndices,
						Name:        uncategorizedFeatureName,
					},
				},
				Name: uncategorizedDomainName,
			},
		}
	}

	return &specview.TaxonomyOutput{Domains: domains}, totalUsage
}

// normalizeDomainName standardizes domain names for matching.
func normalizeDomainName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// mergeFeatures adds features from source domain to target domain with index remapping.
func mergeFeatures(target *specview.TaxonomyDomain, source specview.TaxonomyDomain, offset int) {
	featureMap := make(map[string]int) // feature name -> index in target.Features
	for i, f := range target.Features {
		featureMap[normalizeDomainName(f.Name)] = i
	}

	for _, sf := range source.Features {
		globalIndices := remapIndices(sf.FileIndices, offset)
		normalizedName := normalizeDomainName(sf.Name)

		if idx, ok := featureMap[normalizedName]; ok {
			// Merge into existing feature
			target.Features[idx].FileIndices = append(target.Features[idx].FileIndices, globalIndices...)
		} else {
			// Add new feature
			target.Features = append(target.Features, specview.TaxonomyFeature{
				FileIndices: globalIndices,
				Name:        sf.Name,
			})
			featureMap[normalizedName] = len(target.Features) - 1
		}
	}
}

// remapDomainIndices creates a copy of domain with remapped file indices.
func remapDomainIndices(domain specview.TaxonomyDomain, offset int) *specview.TaxonomyDomain {
	features := make([]specview.TaxonomyFeature, len(domain.Features))
	for i, f := range domain.Features {
		features[i] = specview.TaxonomyFeature{
			FileIndices: remapIndices(f.FileIndices, offset),
			Name:        f.Name,
		}
	}

	return &specview.TaxonomyDomain{
		Description: domain.Description,
		Features:    features,
		Name:        domain.Name,
	}
}

// remapIndices adds offset to all indices.
// Always returns a new slice to avoid aliasing issues.
func remapIndices(indices []int, offset int) []int {
	result := make([]int, len(indices))
	for i, idx := range indices {
		result[i] = idx + offset
	}
	return result
}

// generateHeuristicTaxonomyForChunk creates fallback taxonomy for a failed chunk.
// Returns taxonomy with single Uncategorized domain containing all chunk files.
func generateHeuristicTaxonomyForChunk(result ChunkedTaxonomyResult) *specview.TaxonomyOutput {
	indices := make([]int, result.FileCount)
	for i := range indices {
		indices[i] = i // Local indices, will be remapped by caller
	}

	return &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Description: fmt.Sprintf("Files from chunk %d", result.ChunkIndex),
				Features: []specview.TaxonomyFeature{
					{
						FileIndices: indices,
						Name:        uncategorizedFeatureName,
					},
				},
				Name: uncategorizedDomainName,
			},
		},
	}
}

