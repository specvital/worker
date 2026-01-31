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
// Applies post-processing to validate and normalize classification results.
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

	metricsCollector := NewPhase1MetricsCollector()

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

		// Record raw batch results for metrics (before post-processing normalization)
		metricsCollector.RecordBatch(results, retries, fallbacks)
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

	// Apply post-processing: validate and normalize results
	flatResults := flattenBatchResults(allResults)
	postProcessor := NewPhase1PostProcessor(DefaultPostProcessorConfig())
	processedResults, violations := postProcessor.Process(flatResults, tests)

	if len(violations) > 0 {
		slog.WarnContext(ctx, "phase 1 post-processing found violations",
			"violation_count", len(violations),
		)
	}

	output := mergeV3ResultsFromFlat(processedResults, input)

	metricsCollector.LogMetrics(input.AnalysisID)

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
// Captures and propagates domain descriptions for better anchor context.
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
			// Update description if provided and not already set
			if r.DomainDesc != "" && summary.Description == "" {
				summary.Description = r.DomainDesc
			}
		} else {
			domainMap[r.Domain] = &prompt.DomainSummary{
				Description: r.DomainDesc,
				Name:        r.Domain,
				Features:    []string{r.Feature},
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

// flattenBatchResults converts nested batch results into a flat slice.
func flattenBatchResults(allResults [][]v3BatchResult) []v3BatchResult {
	totalCount := 0
	for _, batch := range allResults {
		totalCount += len(batch)
	}

	flat := make([]v3BatchResult, 0, totalCount)
	for _, batch := range allResults {
		flat = append(flat, batch...)
	}

	return flat
}

// mergeV3ResultsFromFlat converts flat results into Phase1Output.
// Used after post-processing when results are already flattened.
func mergeV3ResultsFromFlat(results []v3BatchResult, input specview.Phase1Input) *specview.Phase1Output {
	// Build flat test list to get index mapping
	tests := flattenTests(input.Files)

	// Handle empty results - create path-based domains
	if len(results) == 0 {
		pathBasedResults := createDomainsFromPaths(tests)
		return buildPhase1Output(pathBasedResults, tests)
	}

	return buildPhase1Output(results, tests)
}

// buildPhase1Output constructs Phase1Output from flat results and test list.
func buildPhase1Output(results []v3BatchResult, tests []specview.TestForAssignment) *specview.Phase1Output {
	// Map domain/feature -> test indices
	domainFeatureMap := make(map[string]map[string][]int)
	// Map domain -> description (first non-empty wins)
	domainDescMap := make(map[string]string)

	for i, r := range results {
		if i >= len(tests) {
			break
		}

		testIdx := tests[i].Index

		if domainFeatureMap[r.Domain] == nil {
			domainFeatureMap[r.Domain] = make(map[string][]int)
		}
		domainFeatureMap[r.Domain][r.Feature] = append(domainFeatureMap[r.Domain][r.Feature], testIdx)

		// Capture first non-empty description for each domain
		if r.DomainDesc != "" && domainDescMap[r.Domain] == "" {
			domainDescMap[r.Domain] = r.DomainDesc
		}
	}

	// Handle empty domain map - should not happen after post-processing
	if len(domainFeatureMap) == 0 {
		return &specview.Phase1Output{Domains: []specview.DomainGroup{}}
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
			Confidence:  defaultClassificationConfidence,
			Description: domainDescMap[domainName],
			Features:    features,
			Name:        domainName,
		})
	}

	return &specview.Phase1Output{Domains: domains}
}

// mergeV3Results converts all batch results into Phase1Output.
// Maps test indices to domain/feature structure and preserves domain descriptions.
// Deprecated: Use mergeV3ResultsFromFlat after post-processing instead.
func mergeV3Results(allResults [][]v3BatchResult, input specview.Phase1Input) *specview.Phase1Output {
	flat := flattenBatchResults(allResults)
	return mergeV3ResultsFromFlat(flat, input)
}
