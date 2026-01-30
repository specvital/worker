package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"

	"github.com/specvital/worker/internal/adapter/ai/prompt"
	"github.com/specvital/worker/internal/adapter/ai/reliability"
	"github.com/specvital/worker/internal/domain/specview"
)

const (
	// uncategorizedDomainName is the fallback domain for unassigned files.
	uncategorizedDomainName = "Uncategorized"
	// uncategorizedFeatureName is the fallback feature for unassigned files.
	uncategorizedFeatureName = "General"
)

// prepareTaxonomyInput converts Phase1Input to TaxonomyInput.
// Extracts file metadata without test names to minimize token usage.
func prepareTaxonomyInput(input specview.Phase1Input) specview.TaxonomyInput {
	files := make([]specview.TaxonomyFileInfo, 0, len(input.Files))

	for i, f := range input.Files {
		files = append(files, specview.TaxonomyFileInfo{
			DomainHints: f.DomainHints,
			Index:       i,
			Path:        f.Path,
			TestCount:   len(f.Tests),
		})
	}

	return specview.TaxonomyInput{
		AnalysisID: input.AnalysisID,
		Files:      files,
		Language:   input.Language,
	}
}

// extractTaxonomy performs Stage 1: taxonomy extraction from file metadata.
// Returns a domain taxonomy structure for Stage 2 test assignment.
func (p *Provider) extractTaxonomy(ctx context.Context, input specview.TaxonomyInput) (*specview.TaxonomyOutput, *specview.TokenUsage, error) {
	if len(input.Files) == 0 {
		return nil, nil, fmt.Errorf("%w: no files for taxonomy extraction", specview.ErrInvalidInput)
	}

	systemPrompt := prompt.Phase1TaxonomySystemPrompt
	userPrompt := prompt.BuildTaxonomyUserPrompt(input)

	var output *specview.TaxonomyOutput
	var usage *specview.TokenUsage

	err := p.phase1Retry.Do(ctx, func() error {
		result, innerUsage, innerErr := p.generateContent(ctx, p.phase1Model, systemPrompt, userPrompt, p.phase1CB)
		if innerErr != nil {
			return innerErr
		}
		usage = innerUsage

		var parseErr error
		output, parseErr = parseTaxonomyResponse(result)
		if parseErr != nil {
			slog.WarnContext(ctx, "failed to parse taxonomy response, will retry",
				"error", parseErr,
				"response", truncateForLog(result, 500),
			)
			return &reliability.RetryableError{Err: parseErr}
		}

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("taxonomy extraction failed: %w", err)
	}

	if output == nil {
		return nil, nil, fmt.Errorf("taxonomy extraction returned nil output")
	}

	fileCount := len(input.Files)
	if err := validateTaxonomy(output, fileCount); err != nil {
		slog.WarnContext(ctx, "taxonomy validation failed, adding missing files to Uncategorized",
			"error", err,
		)
		output = recoverMissingFiles(output, fileCount)
	}

	logAttrs := []any{
		"domain_count", len(output.Domains),
		"file_count", fileCount,
	}
	if usage != nil {
		logAttrs = append(logAttrs, "prompt_tokens", usage.PromptTokens, "output_tokens", usage.CandidatesTokens)
	}
	slog.InfoContext(ctx, "taxonomy extraction complete", logAttrs...)

	return output, usage, nil
}

// parseTaxonomyResponse parses the JSON response into TaxonomyOutput.
func parseTaxonomyResponse(jsonStr string) (*specview.TaxonomyOutput, error) {
	var resp specview.TaxonomyOutput
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	if len(resp.Domains) == 0 {
		return nil, fmt.Errorf("no domains in taxonomy response")
	}

	for i, d := range resp.Domains {
		if d.Name == "" {
			return nil, fmt.Errorf("domain[%d] has empty name", i)
		}
		for j, f := range d.Features {
			if f.Name == "" {
				return nil, fmt.Errorf("domain[%d].feature[%d] has empty name", i, j)
			}
		}
	}

	return &resp, nil
}

// validateTaxonomy checks if all file indices are covered by at least one feature.
// Duplicate assignments (same file in multiple features) are allowed and expected.
func validateTaxonomy(output *specview.TaxonomyOutput, fileCount int) error {
	if output == nil || len(output.Domains) == 0 {
		return fmt.Errorf("no domains in taxonomy output")
	}

	covered := make(map[int]bool)
	for _, domain := range output.Domains {
		for _, feature := range domain.Features {
			for _, idx := range feature.FileIndices {
				if idx < 0 || idx >= fileCount {
					return fmt.Errorf("file index %d out of range [0, %d)", idx, fileCount)
				}
				covered[idx] = true
			}
		}
	}

	if len(covered) < fileCount {
		missing := fileCount - len(covered)
		return fmt.Errorf("%d files not assigned to any feature", missing)
	}

	return nil
}

// collectCoveredIndices builds a map of file indices that are assigned to features.
// Only includes valid indices within [0, fileCount) range.
func collectCoveredIndices(output *specview.TaxonomyOutput, fileCount int) map[int]bool {
	covered := make(map[int]bool)
	for _, domain := range output.Domains {
		for _, feature := range domain.Features {
			for _, idx := range feature.FileIndices {
				if idx >= 0 && idx < fileCount {
					covered[idx] = true
				}
			}
		}
	}
	return covered
}

// recoverMissingFiles adds unassigned files to the Uncategorized domain.
// NOTE: This function mutates the input output directly and returns the same pointer.
func recoverMissingFiles(output *specview.TaxonomyOutput, fileCount int) *specview.TaxonomyOutput {
	covered := collectCoveredIndices(output, fileCount)

	var missingIndices []int
	for i := 0; i < fileCount; i++ {
		if !covered[i] {
			missingIndices = append(missingIndices, i)
		}
	}

	if len(missingIndices) == 0 {
		return output
	}

	sort.Ints(missingIndices)

	uncategorizedIdx := -1
	for i, domain := range output.Domains {
		if domain.Name == uncategorizedDomainName {
			uncategorizedIdx = i
			break
		}
	}

	if uncategorizedIdx >= 0 {
		generalIdx := -1
		for j, feature := range output.Domains[uncategorizedIdx].Features {
			if feature.Name == uncategorizedFeatureName {
				generalIdx = j
				break
			}
		}

		if generalIdx >= 0 {
			output.Domains[uncategorizedIdx].Features[generalIdx].FileIndices = append(
				output.Domains[uncategorizedIdx].Features[generalIdx].FileIndices,
				missingIndices...,
			)
		} else {
			output.Domains[uncategorizedIdx].Features = append(
				output.Domains[uncategorizedIdx].Features,
				specview.TaxonomyFeature{
					FileIndices: missingIndices,
					Name:        uncategorizedFeatureName,
				},
			)
		}
	} else {
		output.Domains = append(output.Domains, specview.TaxonomyDomain{
			Description: "Files that do not fit into specific domains",
			Features: []specview.TaxonomyFeature{
				{
					FileIndices: missingIndices,
					Name:        uncategorizedFeatureName,
				},
			},
			Name: uncategorizedDomainName,
		})
	}

	return output
}
