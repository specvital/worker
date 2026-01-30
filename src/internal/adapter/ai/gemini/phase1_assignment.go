package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/specvital/worker/internal/adapter/ai/prompt"
	"github.com/specvital/worker/internal/adapter/ai/reliability"
	"github.com/specvital/worker/internal/domain/specview"
)

const (
	// assignmentBatchSize is the maximum number of tests per batch.
	// 100 tests keeps prompt under 8K tokens for reliable processing.
	assignmentBatchSize = 100
)

// createAssignmentBatches splits Phase1Input into batches for Stage 2 processing.
// Each batch contains up to 100 tests to stay within token limits.
func createAssignmentBatches(input specview.Phase1Input, batchSize int) []specview.AssignmentBatch {
	if batchSize <= 0 {
		batchSize = assignmentBatchSize
	}

	totalTests := 0
	for _, file := range input.Files {
		totalTests += len(file.Tests)
	}

	allTests := make([]specview.TestForAssignment, 0, totalTests)

	for _, file := range input.Files {
		for _, test := range file.Tests {
			allTests = append(allTests, specview.TestForAssignment{
				FilePath:  file.Path,
				Index:     test.Index,
				Name:      test.Name,
				SuitePath: test.SuitePath,
			})
		}
	}

	if len(allTests) == 0 {
		return nil
	}

	batchCount := (len(allTests) + batchSize - 1) / batchSize
	batches := make([]specview.AssignmentBatch, 0, batchCount)

	for i := 0; i < len(allTests); i += batchSize {
		end := min(i+batchSize, len(allTests))

		batches = append(batches, specview.AssignmentBatch{
			BatchIndex: len(batches),
			Tests:      allTests[i:end],
		})
	}

	return batches
}

// assignTestsBatch assigns a single batch of tests to the fixed taxonomy.
// Uses phase2Model (flash-lite) for cost efficiency.
func (p *Provider) assignTestsBatch(
	ctx context.Context,
	batch specview.AssignmentBatch,
	taxonomy *specview.TaxonomyOutput,
	lang specview.Language,
) (*specview.AssignmentOutput, *specview.TokenUsage, error) {
	if len(batch.Tests) == 0 {
		return &specview.AssignmentOutput{}, nil, nil
	}

	systemPrompt := prompt.Phase1AssignmentSystemPrompt
	userPrompt := prompt.BuildAssignmentUserPrompt(batch, taxonomy, lang)

	var output *specview.AssignmentOutput
	var usage *specview.TokenUsage

	err := p.phase2Retry.Do(ctx, func() error {
		result, innerUsage, innerErr := p.generateContent(ctx, p.phase2Model, systemPrompt, userPrompt, p.phase2CB)
		if innerErr != nil {
			return innerErr
		}
		usage = innerUsage

		var parseErr error
		output, parseErr = parseAssignmentResponse(result)
		if parseErr != nil {
			slog.WarnContext(ctx, "failed to parse assignment response, will retry",
				"error", parseErr,
				"batch_index", batch.BatchIndex,
				"response", truncateForLog(result, 500),
			)
			return &reliability.RetryableError{Err: parseErr}
		}

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("assignment batch %d failed: %w", batch.BatchIndex, err)
	}

	if output == nil {
		return nil, nil, fmt.Errorf("assignment batch %d returned nil output", batch.BatchIndex)
	}

	if err := validateAssignments(ctx, output, batch, taxonomy); err != nil {
		slog.WarnContext(ctx, "assignment validation failed, recovering invalid pairs",
			"error", err,
			"batch_index", batch.BatchIndex,
		)
		output = recoverInvalidAssignments(output, batch, taxonomy)
	}

	return output, usage, nil
}

// parseAssignmentResponse parses the JSON response into AssignmentOutput.
func parseAssignmentResponse(jsonStr string) (*specview.AssignmentOutput, error) {
	var resp specview.AssignmentOutput
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	if len(resp.Assignments) == 0 {
		return nil, fmt.Errorf("no assignments in response")
	}

	for i, a := range resp.Assignments {
		if a.Domain == "" {
			return nil, fmt.Errorf("assignment[%d] has empty domain", i)
		}
		if a.Feature == "" {
			return nil, fmt.Errorf("assignment[%d] has empty feature", i)
		}
		if len(a.TestIndices) == 0 {
			return nil, fmt.Errorf("assignment[%d] has no test indices", i)
		}
	}

	return &resp, nil
}

// validateAssignments validates that all assignments use valid taxonomy names
// and cover all tests in the batch. Returns error if any invalid pairs are found
// or tests are missing, triggering recovery via recoverInvalidAssignments.
func validateAssignments(
	ctx context.Context,
	output *specview.AssignmentOutput,
	batch specview.AssignmentBatch,
	taxonomy *specview.TaxonomyOutput,
) error {
	if output == nil || len(output.Assignments) == 0 {
		return fmt.Errorf("no assignments in output")
	}

	validPairs := buildValidPairs(taxonomy)
	expectedIndices := buildExpectedIndices(batch)
	coveredIndices := make(map[int]bool)
	hasInvalidPairs := false

	for _, assignment := range output.Assignments {
		pair := normalizePairKey(assignment.Domain, assignment.Feature)
		if !validPairs[pair] {
			slog.WarnContext(ctx, "invalid domain/feature pair in assignment",
				"domain", assignment.Domain,
				"feature", assignment.Feature,
			)
			hasInvalidPairs = true
		}

		for _, idx := range assignment.TestIndices {
			if !expectedIndices[idx] {
				return fmt.Errorf("unexpected test index %d in assignment", idx)
			}
			if coveredIndices[idx] {
				slog.WarnContext(ctx, "duplicate test index in assignments",
					"index", idx,
					"domain", assignment.Domain,
					"feature", assignment.Feature,
				)
			}
			coveredIndices[idx] = true
		}
	}

	if hasInvalidPairs {
		return fmt.Errorf("invalid domain/feature pairs found, recovery needed")
	}

	if len(coveredIndices) < len(expectedIndices) {
		missing := len(expectedIndices) - len(coveredIndices)
		return fmt.Errorf("%d tests not assigned", missing)
	}

	return nil
}

// buildExpectedIndices creates a set of expected test indices from batch.
func buildExpectedIndices(batch specview.AssignmentBatch) map[int]bool {
	indices := make(map[int]bool)
	for _, test := range batch.Tests {
		indices[test.Index] = true
	}
	return indices
}

// buildValidPairs constructs a set of valid domain/feature pairs from taxonomy.
// Uses normalized keys (lowercase, trimmed) to handle minor case/whitespace variations.
// Always includes Uncategorized/General as a valid fallback pair.
func buildValidPairs(taxonomy *specview.TaxonomyOutput) map[string]bool {
	pairs := make(map[string]bool)
	pairs[normalizePairKey(uncategorizedDomainName, uncategorizedFeatureName)] = true

	if taxonomy == nil {
		return pairs
	}

	for _, domain := range taxonomy.Domains {
		for _, feature := range domain.Features {
			pairs[normalizePairKey(domain.Name, feature.Name)] = true
		}
	}

	return pairs
}

// normalizePairKey creates a normalized key for domain/feature matching.
// Handles minor case and whitespace variations from AI responses.
func normalizePairKey(domain, feature string) string {
	return strings.ToLower(strings.TrimSpace(domain)) + "/" + strings.ToLower(strings.TrimSpace(feature))
}

// recoverInvalidAssignments fixes invalid assignments by mapping them to Uncategorized.
// Also ensures all tests in batch are assigned. Sorts uncategorized indices for determinism.
func recoverInvalidAssignments(
	output *specview.AssignmentOutput,
	batch specview.AssignmentBatch,
	taxonomy *specview.TaxonomyOutput,
) *specview.AssignmentOutput {
	validPairs := buildValidPairs(taxonomy)
	expectedIndices := buildExpectedIndices(batch)
	coveredIndices := make(map[int]bool)

	validAssignments := make([]specview.TestAssignment, 0, len(output.Assignments)+1)
	var uncategorizedIndices []int

	for _, assignment := range output.Assignments {
		pair := normalizePairKey(assignment.Domain, assignment.Feature)
		isValid := validPairs[pair]

		var validIndices []int
		for _, idx := range assignment.TestIndices {
			if expectedIndices[idx] && !coveredIndices[idx] {
				if isValid {
					validIndices = append(validIndices, idx)
				} else {
					uncategorizedIndices = append(uncategorizedIndices, idx)
				}
				coveredIndices[idx] = true
			}
		}

		if len(validIndices) > 0 {
			validAssignments = append(validAssignments, specview.TestAssignment{
				Domain:      assignment.Domain,
				Feature:     assignment.Feature,
				TestIndices: validIndices,
			})
		}
	}

	// Collect missing indices deterministically by iterating batch order
	for _, test := range batch.Tests {
		if !coveredIndices[test.Index] {
			uncategorizedIndices = append(uncategorizedIndices, test.Index)
		}
	}

	if len(uncategorizedIndices) > 0 {
		sort.Ints(uncategorizedIndices)
		validAssignments = append(validAssignments, specview.TestAssignment{
			Domain:      uncategorizedDomainName,
			Feature:     uncategorizedFeatureName,
			TestIndices: uncategorizedIndices,
		})
	}

	return &specview.AssignmentOutput{
		Assignments: validAssignments,
	}
}
