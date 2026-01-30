package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/specvital/worker/internal/adapter/ai/prompt"
	"github.com/specvital/worker/internal/adapter/ai/reliability"
	"github.com/specvital/worker/internal/domain/specview"
)

const (
	// v3BatchSize is the number of tests processed per batch in V3 architecture.
	// 20 tests produce ~600 output tokens, safely under token limits.
	v3BatchSize = 20

	// v3MaxRetries is the maximum number of retry attempts for batch processing.
	v3MaxRetries = 3

	// v3MinBatchSizeForSplit is the minimum batch size that can be split.
	// Batches smaller than this go directly to individual processing.
	v3MinBatchSizeForSplit = 4
)

// v3BatchResult represents a single classification result from V3 batch processing.
// Uses compact field names to minimize output tokens.
type v3BatchResult struct {
	Domain  string `json:"d"`
	Feature string `json:"f"`
}

// processV3Batch processes a single batch of tests and returns classifications.
// Returns error if API call fails or response validation fails.
func (p *Provider) processV3Batch(
	ctx context.Context,
	tests []specview.TestForAssignment,
	existingDomains []prompt.DomainSummary,
	lang specview.Language,
) ([]v3BatchResult, *specview.TokenUsage, error) {
	if len(tests) == 0 {
		return []v3BatchResult{}, nil, nil
	}

	systemPrompt := prompt.Phase1V3SystemPrompt
	userPrompt := prompt.BuildV3BatchUserPrompt(tests, existingDomains, lang)

	var results []v3BatchResult
	var usage *specview.TokenUsage

	err := p.phase1Retry.Do(ctx, func() error {
		result, innerUsage, innerErr := p.generateContent(ctx, p.phase1Model, systemPrompt, userPrompt, p.phase1CB)
		if innerErr != nil {
			return innerErr
		}
		usage = innerUsage

		parsed, parseErr := parseV3BatchResponse(result)
		if parseErr != nil {
			slog.WarnContext(ctx, "failed to parse v3 batch response, will retry",
				"error", parseErr,
				"response", truncateForLog(result, 500),
			)
			return &reliability.RetryableError{Err: parseErr}
		}

		if err := validateV3BatchCount(parsed, len(tests)); err != nil {
			slog.WarnContext(ctx, "v3 batch count validation failed, will retry",
				"error", err,
				"expected", len(tests),
				"got", len(parsed),
			)
			return &reliability.RetryableError{Err: err}
		}

		results = parsed
		return nil
	})
	if err != nil {
		return nil, usage, fmt.Errorf("v3 batch processing failed: %w", err)
	}

	return results, usage, nil
}

// parseV3BatchResponse parses the JSON array response from V3 batch API.
func parseV3BatchResponse(jsonStr string) ([]v3BatchResult, error) {
	if jsonStr == "" {
		return nil, fmt.Errorf("empty response")
	}

	var results []v3BatchResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	if results == nil {
		return nil, fmt.Errorf("null response array")
	}

	for i, r := range results {
		if r.Domain == "" || r.Feature == "" {
			return nil, fmt.Errorf("empty domain or feature at index %d", i)
		}
	}

	return results, nil
}

// validateV3BatchCount validates that the response count matches expected count.
func validateV3BatchCount(results []v3BatchResult, expectedCount int) error {
	if len(results) != expectedCount {
		return fmt.Errorf("count mismatch: expected %d, got %d", expectedCount, len(results))
	}
	return nil
}

// processV3BatchWithRetry processes a batch with retry, split, and individual fallback.
// Strategy: retry (max 3) -> split batch -> individual processing -> Uncategorized fallback.
func (p *Provider) processV3BatchWithRetry(
	ctx context.Context,
	tests []specview.TestForAssignment,
	existingDomains []prompt.DomainSummary,
	lang specview.Language,
) ([]v3BatchResult, *specview.TokenUsage, int, int, error) {
	totalUsage := &specview.TokenUsage{Model: p.phase1Model}
	retryCount := 0
	fallbackCount := 0

	if len(tests) == 0 {
		return []v3BatchResult{}, totalUsage, 0, 0, nil
	}

	// Try batch processing with retries
	for attempt := 1; attempt <= v3MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, totalUsage, retryCount, fallbackCount, err
		}

		results, usage, err := p.processV3Batch(ctx, tests, existingDomains, lang)
		if usage != nil {
			accumulateTokenUsage(totalUsage, usage)
		}

		if err == nil {
			return results, totalUsage, retryCount, fallbackCount, nil
		}

		retryCount++
		slog.WarnContext(ctx, "v3 batch attempt failed",
			"attempt", attempt,
			"max_attempts", v3MaxRetries,
			"test_count", len(tests),
			"error", err,
		)
	}

	// Batch processing failed - try splitting
	if len(tests) >= v3MinBatchSizeForSplit {
		slog.InfoContext(ctx, "v3 batch retries exhausted, attempting split",
			"test_count", len(tests),
		)

		results, splitUsage, splitRetries, splitFallbacks, err := p.processV3SplitBatch(ctx, tests, existingDomains, lang)
		accumulateTokenUsage(totalUsage, splitUsage)
		retryCount += splitRetries
		fallbackCount += splitFallbacks

		if err == nil {
			return results, totalUsage, retryCount, fallbackCount, nil
		}
	}

	// Split failed or batch too small - fall back to individual processing
	slog.WarnContext(ctx, "v3 batch falling back to individual processing",
		"test_count", len(tests),
	)

	results, indivUsage, indivFallbacks := p.processV3Individual(ctx, tests, existingDomains, lang)
	accumulateTokenUsage(totalUsage, indivUsage)
	fallbackCount += indivFallbacks

	return results, totalUsage, retryCount, fallbackCount, nil
}

// processV3SplitBatch splits a batch and processes each half recursively.
func (p *Provider) processV3SplitBatch(
	ctx context.Context,
	tests []specview.TestForAssignment,
	existingDomains []prompt.DomainSummary,
	lang specview.Language,
) ([]v3BatchResult, *specview.TokenUsage, int, int, error) {
	left, right := splitBatch(tests)
	totalUsage := &specview.TokenUsage{Model: p.phase1Model}
	totalRetries := 0
	totalFallbacks := 0

	// Process left half
	leftResults, leftUsage, leftRetries, leftFallbacks, leftErr := p.processV3BatchWithRetry(ctx, left, existingDomains, lang)
	accumulateTokenUsage(totalUsage, leftUsage)
	totalRetries += leftRetries
	totalFallbacks += leftFallbacks

	if leftErr != nil {
		return nil, totalUsage, totalRetries, totalFallbacks, fmt.Errorf("left split failed: %w", leftErr)
	}

	// Process right half
	rightResults, rightUsage, rightRetries, rightFallbacks, rightErr := p.processV3BatchWithRetry(ctx, right, existingDomains, lang)
	accumulateTokenUsage(totalUsage, rightUsage)
	totalRetries += rightRetries
	totalFallbacks += rightFallbacks

	if rightErr != nil {
		return nil, totalUsage, totalRetries, totalFallbacks, fmt.Errorf("right split failed: %w", rightErr)
	}

	// Combine results
	results := make([]v3BatchResult, 0, len(leftResults)+len(rightResults))
	results = append(results, leftResults...)
	results = append(results, rightResults...)

	return results, totalUsage, totalRetries, totalFallbacks, nil
}

// processV3Individual processes each test individually as final fallback.
// Assigns Uncategorized/General on individual failure.
func (p *Provider) processV3Individual(
	ctx context.Context,
	tests []specview.TestForAssignment,
	existingDomains []prompt.DomainSummary,
	lang specview.Language,
) ([]v3BatchResult, *specview.TokenUsage, int) {
	totalUsage := &specview.TokenUsage{Model: p.phase1Model}
	results := make([]v3BatchResult, 0, len(tests))
	fallbackCount := 0

	for i, test := range tests {
		if err := ctx.Err(); err != nil {
			return results, totalUsage, fallbackCount
		}

		singleTest := []specview.TestForAssignment{test}
		result, usage, err := p.processV3Batch(ctx, singleTest, existingDomains, lang)
		if usage != nil {
			accumulateTokenUsage(totalUsage, usage)
		}

		if err != nil || len(result) != 1 {
			slog.WarnContext(ctx, "v3 individual processing failed, assigning to Uncategorized",
				"test_index", i,
				"test_name", test.Name,
				"error", err,
			)
			results = append(results, v3BatchResult{
				Domain:  uncategorizedDomainName,
				Feature: uncategorizedFeatureName,
			})
			fallbackCount++
			continue
		}

		results = append(results, result[0])
	}

	return results, totalUsage, fallbackCount
}

// splitBatch divides a batch into two halves.
// Requires len(tests) >= 2 for meaningful split; caller must guard with v3MinBatchSizeForSplit.
func splitBatch(tests []specview.TestForAssignment) ([]specview.TestForAssignment, []specview.TestForAssignment) {
	mid := len(tests) / 2
	return tests[:mid], tests[mid:]
}

// accumulateTokenUsage adds usage from source to target.
func accumulateTokenUsage(target, source *specview.TokenUsage) {
	if target == nil || source == nil {
		return
	}
	target.CandidatesTokens += source.CandidatesTokens
	target.PromptTokens += source.PromptTokens
	target.TotalTokens += source.TotalTokens
}
