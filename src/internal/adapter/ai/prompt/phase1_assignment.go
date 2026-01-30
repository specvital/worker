package prompt

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/specvital/worker/internal/domain/specview"
)

//go:embed templates/phase1_assignment_system.md
var Phase1AssignmentSystemPrompt string

// BuildAssignmentUserPrompt builds the user prompt for Stage 2 test assignment.
// It includes the fixed taxonomy and a batch of tests to be assigned.
//
// Token budget: Designed to stay under 8K tokens for 100 tests + taxonomy.
// - Taxonomy: ~50 tokens per feature (20 features max = 1K tokens)
// - Tests: ~30 tokens per test (100 tests = 3K tokens)
// - Overhead: ~500 tokens for structure
func BuildAssignmentUserPrompt(batch specview.AssignmentBatch, taxonomy *specview.TaxonomyOutput, lang specview.Language) string {
	var sb strings.Builder

	sb.WriteString("Assign the following tests to the taxonomy.\n\n")
	fmt.Fprintf(&sb, "Target Language: %s\n\n", lang)

	writeTaxonomySection(&sb, taxonomy)
	writeTestsSection(&sb, batch)

	return sb.String()
}

func writeTaxonomySection(sb *strings.Builder, taxonomy *specview.TaxonomyOutput) {
	sb.WriteString("<taxonomy>\n")

	if taxonomy == nil || len(taxonomy.Domains) == 0 {
		sb.WriteString("(empty)\n")
		sb.WriteString("</taxonomy>\n\n")
		return
	}

	for _, domain := range taxonomy.Domains {
		fmt.Fprintf(sb, "- %s\n", domain.Name)
		for _, feature := range domain.Features {
			fmt.Fprintf(sb, "  - %s\n", feature.Name)
		}
	}

	sb.WriteString("</taxonomy>\n\n")

	// Write explicit valid pairs to prevent AI hallucination
	writeValidPairs(sb, taxonomy)
}

// writeValidPairs explicitly lists all valid domain/feature combinations.
// This prevents AI from inventing new feature names not in the taxonomy.
func writeValidPairs(sb *strings.Builder, taxonomy *specview.TaxonomyOutput) {
	sb.WriteString("<valid-pairs>\n")
	sb.WriteString("Use ONLY these exact domain/feature combinations:\n")

	pairNum := 1
	for _, domain := range taxonomy.Domains {
		for _, feature := range domain.Features {
			fmt.Fprintf(sb, "%d. \"%s\" / \"%s\"\n", pairNum, domain.Name, feature.Name)
			pairNum++
		}
	}

	// Always include Uncategorized/General as valid
	fmt.Fprintf(sb, "%d. \"Uncategorized\" / \"General\"\n", pairNum)
	sb.WriteString("</valid-pairs>\n\n")
}

func writeTestsSection(sb *strings.Builder, batch specview.AssignmentBatch) {
	sb.WriteString("<tests>\n")

	if len(batch.Tests) == 0 {
		sb.WriteString("</tests>\n\nTotal: 0 tests. No tests to assign.")
		return
	}

	for _, test := range batch.Tests {
		writeTestEntry(sb, test)
	}

	sb.WriteString("</tests>\n\n")

	totalTests := len(batch.Tests)
	if totalTests == 1 {
		sb.WriteString("Total: 1 test (index 0). Assign this test to exactly one feature.")
	} else {
		minIdx, maxIdx := findIndexRange(batch.Tests)
		fmt.Fprintf(sb, "Total: %d tests (indices %d-%d). Assign EVERY TEST to exactly one feature.", totalTests, minIdx, maxIdx)
	}
}

func writeTestEntry(sb *strings.Builder, test specview.TestForAssignment) {
	fmt.Fprintf(sb, "[%d] %s: %s", test.Index, test.FilePath, test.Name)

	if test.SuitePath != "" {
		fmt.Fprintf(sb, " (suite: %s)", test.SuitePath)
	}

	sb.WriteString("\n")
}

func findIndexRange(tests []specview.TestForAssignment) (min, max int) {
	if len(tests) == 0 {
		return 0, 0
	}

	min, max = tests[0].Index, tests[0].Index
	for _, t := range tests[1:] {
		if t.Index < min {
			min = t.Index
		}
		if t.Index > max {
			max = t.Index
		}
	}
	return min, max
}
