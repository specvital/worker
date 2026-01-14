package prompt

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/specvital/worker/internal/domain/specview"
)

//go:embed templates/phase1_system.md
var Phase1SystemPrompt string

// BuildPhase1UserPrompt builds the user prompt for Phase 1 classification.
func BuildPhase1UserPrompt(input specview.Phase1Input, language specview.Language) string {
	var sb strings.Builder

	sb.WriteString("Classify the following tests into business domains and features.\n\n")
	sb.WriteString(fmt.Sprintf("Target Language: %s\n\n", language))
	sb.WriteString("<files>\n")

	totalTests := 0
	for fileIdx, file := range input.Files {
		sb.WriteString(fmt.Sprintf("[%d] %s", fileIdx, file.Path))
		if file.Framework != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", file.Framework))
		}
		sb.WriteString("\n")

		// Domain hints (imports and calls)
		if file.DomainHints != nil {
			if len(file.DomainHints.Imports) > 0 {
				sb.WriteString(fmt.Sprintf("  imports: %s\n", strings.Join(file.DomainHints.Imports, ", ")))
			}
			if len(file.DomainHints.Calls) > 0 {
				sb.WriteString(fmt.Sprintf("  calls: %s\n", strings.Join(file.DomainHints.Calls, ", ")))
			}
		}

		// Tests
		sb.WriteString("  tests:\n")
		for _, test := range file.Tests {
			if test.SuitePath != "" {
				sb.WriteString(fmt.Sprintf("    %d|%s|%s\n", test.Index, test.SuitePath, test.Name))
			} else {
				sb.WriteString(fmt.Sprintf("    %d|%s\n", test.Index, test.Name))
			}
			totalTests++
		}
	}

	sb.WriteString("</files>\n\n")
	sb.WriteString(fmt.Sprintf("Total: %d tests (indices 0-%d). Assign ALL to exactly one feature.", totalTests, totalTests-1))

	return sb.String()
}

// BuildPhase1UserPromptWithAnchors builds the user prompt for Phase 1 classification
// with anchor domains from previous chunks. This ensures domain naming consistency
// across chunks.
func BuildPhase1UserPromptWithAnchors(input specview.Phase1Input, language specview.Language, anchors []specview.DomainGroup) string {
	var sb strings.Builder

	sb.WriteString("Classify the following tests into business domains and features.\n\n")
	sb.WriteString(fmt.Sprintf("Target Language: %s\n\n", language))

	// Add anchor domains section
	sb.WriteString("## Existing Domains (MUST reuse if applicable)\n\n")
	sb.WriteString("The following domains were identified from previous test batches. ")
	sb.WriteString("You MUST reuse these domain names exactly if the new tests belong to the same business area.\n\n")
	sb.WriteString("<anchor_domains>\n")
	for _, domain := range anchors {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", domain.Name, domain.Description))
		if len(domain.Features) > 0 {
			sb.WriteString("  Features: ")
			featureNames := make([]string, 0, len(domain.Features))
			for _, f := range domain.Features {
				featureNames = append(featureNames, f.Name)
			}
			sb.WriteString(strings.Join(featureNames, ", "))
			sb.WriteString("\n")
		}
	}
	sb.WriteString("</anchor_domains>\n\n")

	sb.WriteString("## Rules\n")
	sb.WriteString("1. If a test matches an existing domain, you MUST use that exact domain name\n")
	sb.WriteString("2. Only create a NEW domain if the test covers a completely new business area\n")
	sb.WriteString("3. Feature names can be new even within existing domains\n\n")

	sb.WriteString("<files>\n")

	totalTests := 0
	for fileIdx, file := range input.Files {
		sb.WriteString(fmt.Sprintf("[%d] %s", fileIdx, file.Path))
		if file.Framework != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", file.Framework))
		}
		sb.WriteString("\n")

		if file.DomainHints != nil {
			if len(file.DomainHints.Imports) > 0 {
				sb.WriteString(fmt.Sprintf("  imports: %s\n", strings.Join(file.DomainHints.Imports, ", ")))
			}
			if len(file.DomainHints.Calls) > 0 {
				sb.WriteString(fmt.Sprintf("  calls: %s\n", strings.Join(file.DomainHints.Calls, ", ")))
			}
		}

		sb.WriteString("  tests:\n")
		for _, test := range file.Tests {
			if test.SuitePath != "" {
				sb.WriteString(fmt.Sprintf("    %d|%s|%s\n", test.Index, test.SuitePath, test.Name))
			} else {
				sb.WriteString(fmt.Sprintf("    %d|%s\n", test.Index, test.Name))
			}
			totalTests++
		}
	}

	sb.WriteString("</files>\n\n")
	sb.WriteString(fmt.Sprintf("Total: %d tests (indices 0-%d). Assign ALL to exactly one feature.", totalTests, totalTests-1))

	return sb.String()
}

