package prompt

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/specvital/worker/internal/domain/specview"
)

//go:embed templates/phase1_taxonomy_system.md
var Phase1TaxonomySystemPrompt string

const (
	// MaxDomainHintsPerType limits imports/calls to prevent token explosion.
	MaxDomainHintsPerType = 10
)

// BuildTaxonomyUserPrompt builds the user prompt for Stage 1 taxonomy extraction.
// It includes only file metadata (paths, hints, test counts) without individual test names.
//
// Token budget: Designed to stay under 15K tokens for 500 files (~30 tokens per file).
// DomainHints are limited to 10 imports + 10 calls per file to ensure budget compliance.
func BuildTaxonomyUserPrompt(input specview.TaxonomyInput) string {
	var sb strings.Builder

	sb.WriteString("Extract domain taxonomy from the following test files.\n\n")
	fmt.Fprintf(&sb, "Target Language: %s\n\n", input.Language)
	sb.WriteString("<files>\n")

	totalFiles := len(input.Files)
	if totalFiles == 0 {
		sb.WriteString("</files>\n\nTotal: 0 files. No files to process.")
		return sb.String()
	}

	for _, file := range input.Files {
		fmt.Fprintf(&sb, "[%d] %s (%d tests)\n", file.Index, file.Path, file.TestCount)

		if file.DomainHints != nil {
			if len(file.DomainHints.Imports) > 0 {
				imports := limitSlice(file.DomainHints.Imports, MaxDomainHintsPerType)
				fmt.Fprintf(&sb, "  imports: %s\n", strings.Join(imports, ", "))
			}
			if len(file.DomainHints.Calls) > 0 {
				calls := limitSlice(file.DomainHints.Calls, MaxDomainHintsPerType)
				fmt.Fprintf(&sb, "  calls: %s\n", strings.Join(calls, ", "))
			}
		}
	}

	sb.WriteString("</files>\n\n")

	if totalFiles == 1 {
		sb.WriteString("Total: 1 file (index 0). Ensure this file appears in at least one feature.")
	} else {
		fmt.Fprintf(&sb, "Total: %d files (indices 0-%d). Ensure EVERY file appears in at least one feature. Files testing multiple concerns may appear in multiple features.", totalFiles, totalFiles-1)
	}

	return sb.String()
}

func limitSlice(slice []string, max int) []string {
	if len(slice) <= max {
		return slice
	}
	return slice[:max]
}
