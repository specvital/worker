package gemini

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"

	"github.com/specvital/worker/internal/adapter/ai/prompt"
	"github.com/specvital/worker/internal/domain/specview"
)

// classifyDomainsV3 performs Phase 1 V3: sequential batch classification.
// Splits all tests into batches of v3BatchSize and processes them sequentially.
// Uses anchor domain propagation to maintain consistency across batches.
func (p *Provider) classifyDomainsV3(ctx context.Context, input specview.Phase1Input, lang specview.Language) (*specview.Phase1Output, *specview.TokenUsage, error) {
	tests := flattenTests(input.Files)
	if len(tests) == 0 {
		return nil, nil, fmt.Errorf("%w: no tests to classify", specview.ErrInvalidInput)
	}

	slog.InfoContext(ctx, "starting phase 1 v3 sequential batch classification",
		"file_count", len(input.Files),
		"test_count", len(tests),
	)

	batches := splitIntoBatches(tests, v3BatchSize)
	totalUsage := &specview.TokenUsage{Model: p.phase1Model}
	totalRetries := 0
	totalFallbacks := 0

	var allResults [][]v3BatchResult
	var existingDomains []prompt.DomainSummary

	for batchIdx, batch := range batches {
		if err := ctx.Err(); err != nil {
			return nil, totalUsage, err
		}

		slog.InfoContext(ctx, "processing v3 batch",
			"batch_index", batchIdx,
			"batch_count", len(batches),
			"test_count", len(batch),
			"existing_domains", len(existingDomains),
		)

		results, usage, retries, fallbacks, err := p.processV3BatchWithRetry(ctx, batch, existingDomains, lang)
		accumulateTokenUsage(totalUsage, usage)
		totalRetries += retries
		totalFallbacks += fallbacks

		if err != nil {
			return nil, totalUsage, fmt.Errorf("batch %d/%d failed: %w", batchIdx+1, len(batches), err)
		}

		allResults = append(allResults, results)

		// Propagate anchor domains to next batch
		existingDomains = extractDomainSummaries(results, existingDomains)

		slog.InfoContext(ctx, "v3 batch processed",
			"batch_index", batchIdx,
			"test_count", len(batch),
			"prompt_tokens", usage.PromptTokens,
			"output_tokens", usage.CandidatesTokens,
			"retry_count", retries,
		)
	}

	output := mergeV3Results(allResults, input)

	slog.InfoContext(ctx, "phase 1 v3 classification complete",
		"total_batches", len(batches),
		"total_tests", len(tests),
		"total_domains", len(output.Domains),
		"total_prompt_tokens", totalUsage.PromptTokens,
		"total_output_tokens", totalUsage.CandidatesTokens,
		"total_retries", totalRetries,
		"total_fallbacks", totalFallbacks,
	)

	return output, totalUsage, nil
}

// flattenTests extracts all tests from file infos into a flat slice.
// Preserves global test index for cross-referencing in output.
func flattenTests(files []specview.FileInfo) []specview.TestForAssignment {
	totalTests := 0
	for _, file := range files {
		totalTests += len(file.Tests)
	}

	tests := make([]specview.TestForAssignment, 0, totalTests)
	for _, file := range files {
		for _, test := range file.Tests {
			tests = append(tests, specview.TestForAssignment{
				FilePath:  file.Path,
				Index:     test.Index,
				Name:      test.Name,
				SuitePath: test.SuitePath,
			})
		}
	}

	return tests
}

// splitIntoBatches divides tests into batches of specified size.
func splitIntoBatches(tests []specview.TestForAssignment, batchSize int) [][]specview.TestForAssignment {
	if len(tests) == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = v3BatchSize
	}

	batchCount := (len(tests) + batchSize - 1) / batchSize
	batches := make([][]specview.TestForAssignment, 0, batchCount)

	for i := 0; i < len(tests); i += batchSize {
		end := min(i+batchSize, len(tests))
		batches = append(batches, tests[i:end])
	}

	return batches
}

// extractDomainSummaries builds domain summaries from batch results for anchor propagation.
// Merges new results with existing domains to build cumulative context.
func extractDomainSummaries(results []v3BatchResult, existing []prompt.DomainSummary) []prompt.DomainSummary {
	// Build domain -> features map from existing
	domainMap := make(map[string]*prompt.DomainSummary)
	for i := range existing {
		domainMap[existing[i].Name] = &existing[i]
	}

	// Add new results
	for _, r := range results {
		if summary, exists := domainMap[r.Domain]; exists {
			if !slices.Contains(summary.Features, r.Feature) {
				summary.Features = append(summary.Features, r.Feature)
			}
		} else {
			domainMap[r.Domain] = &prompt.DomainSummary{
				Name:     r.Domain,
				Features: []string{r.Feature},
			}
		}
	}

	// Convert back to slice with deterministic ordering
	domainNames := make([]string, 0, len(domainMap))
	for name := range domainMap {
		domainNames = append(domainNames, name)
	}
	sort.Strings(domainNames)

	summaries := make([]prompt.DomainSummary, 0, len(domainMap))
	for _, name := range domainNames {
		summary := domainMap[name]
		sort.Strings(summary.Features)
		summaries = append(summaries, *summary)
	}

	return summaries
}

// mergeV3Results converts all batch results into Phase1Output.
// Maps test indices to domain/feature structure.
func mergeV3Results(allResults [][]v3BatchResult, input specview.Phase1Input) *specview.Phase1Output {
	// Build flat test list to get index mapping
	tests := flattenTests(input.Files)

	// Map domain/feature -> test indices
	domainFeatureMap := make(map[string]map[string][]int)

	resultIdx := 0
	for _, batchResults := range allResults {
		for _, r := range batchResults {
			if resultIdx >= len(tests) {
				break
			}

			testIdx := tests[resultIdx].Index

			if domainFeatureMap[r.Domain] == nil {
				domainFeatureMap[r.Domain] = make(map[string][]int)
			}
			domainFeatureMap[r.Domain][r.Feature] = append(domainFeatureMap[r.Domain][r.Feature], testIdx)

			resultIdx++
		}
	}

	// Handle empty results
	if len(domainFeatureMap) == 0 {
		return &specview.Phase1Output{
			Domains: []specview.DomainGroup{
				{
					Confidence:  defaultClassificationConfidence,
					Description: "Files that do not fit into specific domains",
					Features: []specview.FeatureGroup{
						{
							Confidence:  defaultClassificationConfidence,
							Name:        uncategorizedFeatureName,
							TestIndices: collectAllTestIndices(input),
						},
					},
					Name: uncategorizedDomainName,
				},
			},
		}
	}

	// Convert map to output structure with deterministic ordering
	domainNames := make([]string, 0, len(domainFeatureMap))
	for name := range domainFeatureMap {
		domainNames = append(domainNames, name)
	}
	sort.Strings(domainNames)

	domains := make([]specview.DomainGroup, 0, len(domainFeatureMap))
	for _, domainName := range domainNames {
		featureMap := domainFeatureMap[domainName]

		featureNames := make([]string, 0, len(featureMap))
		for name := range featureMap {
			featureNames = append(featureNames, name)
		}
		sort.Strings(featureNames)

		features := make([]specview.FeatureGroup, 0, len(featureMap))
		for _, featureName := range featureNames {
			indices := featureMap[featureName]
			sort.Ints(indices)
			features = append(features, specview.FeatureGroup{
				Confidence:  defaultClassificationConfidence,
				Name:        featureName,
				TestIndices: indices,
			})
		}

		domains = append(domains, specview.DomainGroup{
			Confidence: defaultClassificationConfidence,
			Features:   features,
			Name:       domainName,
		})
	}

	return &specview.Phase1Output{Domains: domains}
}
