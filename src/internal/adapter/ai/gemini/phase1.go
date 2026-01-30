package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/specvital/worker/internal/adapter/ai/prompt"
	"github.com/specvital/worker/internal/adapter/ai/reliability"
	"github.com/specvital/worker/internal/domain/specview"
	"golang.org/x/sync/errgroup"
)

const (
	// waveConcurrency is the number of chunks processed in parallel per wave.
	// Kept at 5 to stay within Gemini rate limits while maximizing throughput.
	waveConcurrency = 5
)

// phase1Response represents the expected JSON response from Phase 1.
type phase1Response struct {
	Domains []phase1Domain `json:"domains"`
}

type phase1Domain struct {
	Confidence  float64         `json:"confidence"`
	Description string          `json:"description"`
	Features    []phase1Feature `json:"features"`
	Name        string          `json:"name"`
}

type phase1Feature struct {
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
	Name        string  `json:"name"`
	TestIndices []int   `json:"test_indices"`
}

// classifyDomains performs Phase 1: domain and feature classification.
// Automatically chunks large inputs to avoid API rate limits.
func (p *Provider) classifyDomains(ctx context.Context, input specview.Phase1Input, lang specview.Language) (*specview.Phase1Output, *specview.TokenUsage, error) {
	if len(input.Files) == 0 {
		return nil, nil, fmt.Errorf("%w: no files to classify", specview.ErrInvalidInput)
	}

	config := DefaultChunkConfig()
	if NeedsChunking(input.Files, config) {
		return p.classifyDomainsChunked(ctx, input, lang, config)
	}

	return p.classifyDomainsSingle(ctx, input, lang, nil)
}

// classifyDomainsSingle performs Phase 1 classification for a single chunk.
// anchorDomains is optional context from previous chunks.
// Retries on both API errors and JSON parsing errors.
func (p *Provider) classifyDomainsSingle(ctx context.Context, input specview.Phase1Input, lang specview.Language, anchorDomains []specview.DomainGroup) (*specview.Phase1Output, *specview.TokenUsage, error) {
	systemPrompt := prompt.Phase1SystemPrompt
	var userPrompt string
	if len(anchorDomains) > 0 {
		userPrompt = prompt.BuildPhase1UserPromptWithAnchors(input, lang, anchorDomains)
	} else {
		userPrompt = prompt.BuildPhase1UserPrompt(input, lang)
	}

	var output *specview.Phase1Output
	var usage *specview.TokenUsage

	err := p.phase1Retry.Do(ctx, func() error {
		// API call
		result, innerUsage, innerErr := p.generateContent(ctx, p.phase1Model, systemPrompt, userPrompt, p.phase1CB)
		if innerErr != nil {
			return innerErr
		}
		usage = innerUsage

		// Parse response - parsing errors are also retryable
		var parseErr error
		output, parseErr = parsePhase1Response(result)
		if parseErr != nil {
			slog.WarnContext(ctx, "failed to parse phase 1 response, will retry",
				"error", parseErr,
				"response", truncateForLog(result, 500),
			)
			// Wrap as RetryableError so retry logic will attempt again
			return &reliability.RetryableError{Err: parseErr}
		}

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("phase 1 classification failed: %w", err)
	}

	if err := validatePhase1Output(ctx, output, input); err != nil {
		slog.WarnContext(ctx, "phase 1 output validation failed",
			"error", err,
		)
		return nil, nil, fmt.Errorf("phase 1 output validation failed: %w", err)
	}

	return output, usage, nil
}

// chunkResult holds the result of processing a single chunk.
type chunkResult struct {
	index  int
	output *specview.Phase1Output
	usage  *specview.TokenUsage
}

// classifyDomainsChunked handles Phase 1 classification for large inputs.
// Uses wave parallel processing: first chunk establishes anchors, remaining chunks
// are processed in parallel waves of waveConcurrency chunks each.
// Supports resumption from cached progress on job retry.
func (p *Provider) classifyDomainsChunked(ctx context.Context, input specview.Phase1Input, lang specview.Language, config ChunkConfig) (*specview.Phase1Output, *specview.TokenUsage, error) {
	chunks := SplitIntoChunks(input.Files, config)

	// Generate cache key from analysisID (more reliable than content hash)
	cacheKey := ChunkCacheKey{
		ContentHash: input.AnalysisID, // Using analysisID for reliable cache key
		Language:    lang,
		ModelID:     p.phase1Model,
	}

	// Check for cached progress from previous attempt
	cache := GetGlobalChunkCache()
	progress := cache.Get(cacheKey)

	var allOutputs []*specview.Phase1Output
	var anchorDomains []specview.DomainGroup
	totalUsage := &specview.TokenUsage{Model: p.phase1Model}
	startChunk := 0

	// Resume from cached progress if available and valid
	if progress != nil && progress.TotalChunks == len(chunks) && progress.CompletedChunks > 0 {
		allOutputs = progress.CompletedOutputs
		anchorDomains = progress.AnchorDomains
		totalUsage = progress.TotalUsage
		startChunk = progress.CompletedChunks

		slog.InfoContext(ctx, "resuming phase 1 from cached progress",
			"total_chunks", len(chunks),
			"completed_chunks", startChunk,
			"remaining_chunks", len(chunks)-startChunk,
		)
	} else {
		slog.InfoContext(ctx, "processing phase 1 in chunks with wave parallelism",
			"total_chunks", len(chunks),
			"total_tests", countTests(input.Files),
			"wave_concurrency", waveConcurrency,
		)
	}

	// Process first chunk sequentially to establish anchor domains (if not resuming)
	if startChunk == 0 && len(chunks) > 0 {
		output, usage, err := p.processChunk(ctx, chunks[0], 0, lang, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("anchor chunk failed: %w", err)
		}

		allOutputs = append(allOutputs, output)
		anchorDomains = output.Domains
		if usage != nil {
			totalUsage.CandidatesTokens += usage.CandidatesTokens
			totalUsage.PromptTokens += usage.PromptTokens
			totalUsage.TotalTokens += usage.TotalTokens
		}
		startChunk = 1
	}

	// Process remaining chunks in parallel waves
	for waveStart := startChunk; waveStart < len(chunks); waveStart += waveConcurrency {
		waveEnd := min(waveStart+waveConcurrency, len(chunks))
		waveNum := (waveStart / waveConcurrency) + 1
		totalWaves := (len(chunks) + waveConcurrency - 1) / waveConcurrency

		slog.InfoContext(ctx, "starting wave",
			"wave", waveNum,
			"total_waves", totalWaves,
			"chunks_in_wave", waveEnd-waveStart,
			"chunk_range", fmt.Sprintf("%d-%d", waveStart+1, waveEnd),
		)

		waveStartTime := time.Now()
		waveResults, err := p.processWave(ctx, chunks[waveStart:waveEnd], waveStart, lang, anchorDomains)
		if err != nil {
			// Save progress before returning error for potential retry
			if len(allOutputs) > 0 {
				cache.Save(cacheKey, &ChunkProgress{
					AnchorDomains:    anchorDomains,
					CompletedChunks:  waveStart,
					CompletedOutputs: allOutputs,
					TotalChunks:      len(chunks),
					TotalUsage:       totalUsage,
				})
				slog.InfoContext(ctx, "saved chunk progress for retry",
					"completed_chunks", waveStart,
					"total_chunks", len(chunks),
				)
			}
			return nil, nil, err
		}

		// Collect wave results and update totals
		for _, result := range waveResults {
			allOutputs = append(allOutputs, result.output)
			if result.usage != nil {
				totalUsage.CandidatesTokens += result.usage.CandidatesTokens
				totalUsage.PromptTokens += result.usage.PromptTokens
				totalUsage.TotalTokens += result.usage.TotalTokens
			}
		}

		// Merge anchor domains after wave completes
		waveOutputs := make([]*specview.Phase1Output, len(waveResults)+1)
		waveOutputs[0] = &specview.Phase1Output{Domains: anchorDomains}
		for i, result := range waveResults {
			waveOutputs[i+1] = result.output
		}
		merged := MergePhase1Outputs(waveOutputs)
		anchorDomains = merged.Domains

		slog.InfoContext(ctx, "wave complete",
			"wave", waveNum,
			"elapsed", time.Since(waveStartTime).Round(time.Millisecond),
			"domains_after_merge", len(anchorDomains),
		)
	}

	// Clear cache on successful completion
	cache.Delete(cacheKey)

	mergedOutput := MergePhase1Outputs(allOutputs)

	slog.InfoContext(ctx, "phase 1 chunked processing complete",
		"total_domains", len(mergedOutput.Domains),
		"total_tokens", totalUsage.TotalTokens,
	)

	return mergedOutput, totalUsage, nil
}

// processWave processes multiple chunks in parallel within a single wave.
// Returns results in chunk order for consistent merging.
func (p *Provider) processWave(ctx context.Context, chunks []ChunkedInput, baseIndex int, lang specview.Language, anchorDomains []specview.DomainGroup) ([]chunkResult, error) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(waveConcurrency)

	results := make([]chunkResult, len(chunks))
	var mu sync.Mutex

	for i, chunk := range chunks {
		chunkIndex := baseIndex + i

		g.Go(func() error {
			output, usage, err := p.processChunk(gctx, chunk, chunkIndex, lang, anchorDomains)
			if err != nil {
				return fmt.Errorf("chunk %d failed: %w", chunkIndex+1, err)
			}

			mu.Lock()
			results[i] = chunkResult{
				index:  chunkIndex,
				output: output,
				usage:  usage,
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// processChunk processes a single chunk and returns the result with restored indices.
func (p *Provider) processChunk(ctx context.Context, chunk ChunkedInput, chunkIndex int, lang specview.Language, anchorDomains []specview.DomainGroup) (*specview.Phase1Output, *specview.TokenUsage, error) {
	chunkStartTime := time.Now()

	slog.InfoContext(ctx, "processing chunk",
		"chunk", chunkIndex+1,
		"tests_in_chunk", countTests(chunk.Files),
	)

	// Reindex tests within chunk to start from 0
	reindexedFiles, indexMap := ReindexTests(chunk.Files)
	chunkInput := specview.Phase1Input{
		Files:    reindexedFiles,
		Language: lang,
	}

	// Process chunk
	output, usage, err := p.classifyDomainsSingle(ctx, chunkInput, lang, anchorDomains)
	if err != nil {
		return nil, nil, err
	}

	// Restore original indices
	RestoreIndices(output, indexMap)

	slog.InfoContext(ctx, "chunk processing complete",
		"chunk", chunkIndex+1,
		"elapsed", time.Since(chunkStartTime).Round(time.Millisecond),
	)

	return output, usage, nil
}

// parsePhase1Response parses the JSON response into Phase1Output.
func parsePhase1Response(jsonStr string) (*specview.Phase1Output, error) {
	var resp phase1Response
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	output := &specview.Phase1Output{
		Domains: make([]specview.DomainGroup, 0, len(resp.Domains)),
	}

	for _, d := range resp.Domains {
		domain := specview.DomainGroup{
			Confidence:  d.Confidence,
			Description: d.Description,
			Features:    make([]specview.FeatureGroup, 0, len(d.Features)),
			Name:        d.Name,
		}

		for _, f := range d.Features {
			feature := specview.FeatureGroup{
				Confidence:  f.Confidence,
				Description: f.Description,
				Name:        f.Name,
				TestIndices: f.TestIndices,
			}
			domain.Features = append(domain.Features, feature)
		}

		output.Domains = append(output.Domains, domain)
	}

	return output, nil
}

const (
	// uncategorizedDomainName is the domain name for auto-recovered missing indices.
	uncategorizedDomainName = "Uncategorized"

	// uncategorizedFeatureName is the feature name for auto-recovered missing indices.
	uncategorizedFeatureName = "Uncategorized Tests"

	// missingIndexWarningThreshold is the percentage of missing indices that triggers a warning.
	// If more than 5% of indices are missing, a warning is logged.
	missingIndexWarningThreshold = 0.05
)

// validatePhase1Output validates the Phase 1 output against input.
// Auto-assigns missing indices to "Uncategorized" domain instead of failing.
// Filters out unexpected indices (out-of-range) with a warning.
func validatePhase1Output(ctx context.Context, output *specview.Phase1Output, input specview.Phase1Input) error {
	if output == nil || len(output.Domains) == 0 {
		return fmt.Errorf("no domains in output")
	}

	// Collect all test indices from input
	expectedIndices := make(map[int]bool)
	for _, file := range input.Files {
		for _, test := range file.Tests {
			expectedIndices[test.Index] = true
		}
	}

	// Collect valid test indices from output, filtering out unexpected ones
	coveredIndices := make(map[int]bool)
	var unexpectedIndices []int
	for i := range output.Domains {
		if output.Domains[i].Name == "" {
			return fmt.Errorf("domain name is empty")
		}
		for j := range output.Domains[i].Features {
			if output.Domains[i].Features[j].Name == "" {
				return fmt.Errorf("feature name is empty in domain %q", output.Domains[i].Name)
			}
			// Filter test indices, keeping only valid ones
			validIndices := make([]int, 0, len(output.Domains[i].Features[j].TestIndices))
			for _, idx := range output.Domains[i].Features[j].TestIndices {
				if expectedIndices[idx] {
					validIndices = append(validIndices, idx)
					coveredIndices[idx] = true
				} else {
					unexpectedIndices = append(unexpectedIndices, idx)
				}
			}
			output.Domains[i].Features[j].TestIndices = validIndices
		}
	}

	// Log warning if unexpected indices were found and filtered
	if len(unexpectedIndices) > 0 {
		sort.Ints(unexpectedIndices)
		slog.WarnContext(ctx, "filtered out unexpected test indices from AI output",
			"unexpected_indices", unexpectedIndices,
			"expected_range", fmt.Sprintf("0-%d", len(expectedIndices)-1),
		)
	}

	// Find missing indices
	var missingIndices []int
	for idx := range expectedIndices {
		if !coveredIndices[idx] {
			missingIndices = append(missingIndices, idx)
		}
	}

	// Auto-recover missing indices by assigning to Uncategorized domain
	if len(missingIndices) > 0 {
		sort.Ints(missingIndices)
		missingRatio := float64(len(missingIndices)) / float64(len(expectedIndices))

		// Log warning if more than threshold are missing
		if missingRatio > missingIndexWarningThreshold {
			slog.WarnContext(ctx, "significant portion of test indices missing, auto-recovering",
				"expected", len(expectedIndices),
				"covered", len(coveredIndices),
				"missing", len(missingIndices),
				"missing_ratio", fmt.Sprintf("%.1f%%", missingRatio*100),
			)
		} else {
			slog.InfoContext(ctx, "auto-recovering missing test indices",
				"missing_count", len(missingIndices),
			)
		}

		// Add missing indices to Uncategorized domain
		addUncategorizedDomain(output, missingIndices)
	}

	return nil
}

// addUncategorizedDomain adds missing indices to the Uncategorized domain.
// If the domain already exists, appends to its feature; otherwise creates it.
func addUncategorizedDomain(output *specview.Phase1Output, missingIndices []int) {
	// Look for existing Uncategorized domain
	for i := range output.Domains {
		if output.Domains[i].Name == uncategorizedDomainName {
			// Find existing Uncategorized Tests feature or add new one
			for j := range output.Domains[i].Features {
				if output.Domains[i].Features[j].Name == uncategorizedFeatureName {
					output.Domains[i].Features[j].TestIndices = append(
						output.Domains[i].Features[j].TestIndices,
						missingIndices...,
					)
					return
				}
			}
			// Uncategorized domain exists but no Uncategorized Tests feature
			output.Domains[i].Features = append(output.Domains[i].Features, specview.FeatureGroup{
				Confidence:  0.5,
				Description: "Tests that could not be classified by AI",
				Name:        uncategorizedFeatureName,
				TestIndices: missingIndices,
			})
			return
		}
	}

	// Create new Uncategorized domain
	output.Domains = append(output.Domains, specview.DomainGroup{
		Confidence:  0.5,
		Description: "Tests that could not be classified by AI",
		Name:        uncategorizedDomainName,
		Features: []specview.FeatureGroup{
			{
				Confidence:  0.5,
				Description: "Tests that could not be classified by AI",
				Name:        uncategorizedFeatureName,
				TestIndices: missingIndices,
			},
		},
	})
}

// truncateForLog truncates a string for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
