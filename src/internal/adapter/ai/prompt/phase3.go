package prompt

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/specvital/worker/internal/domain/specview"
)

//go:embed templates/phase3_system.md
var Phase3SystemPrompt string

// BuildPhase3UserPrompt builds the user prompt for Phase 3 executive summary generation.
func BuildPhase3UserPrompt(input specview.Phase3Input) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Target Language: %s\n\n", input.Language)
	sb.WriteString("<document_structure>\n")

	totalBehaviors := 0
	for _, domain := range input.Domains {
		fmt.Fprintf(&sb, "## %s\n", domain.Name)
		fmt.Fprintf(&sb, "%s\n\n", domain.Description)

		for _, feature := range domain.Features {
			fmt.Fprintf(&sb, "### %s\n", feature.Name)
			fmt.Fprintf(&sb, "%s\n", feature.Description)
			sb.WriteString("Behaviors:\n")

			for _, behavior := range feature.Behaviors {
				fmt.Fprintf(&sb, "- %s\n", behavior.Description)
				totalBehaviors++
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("</document_structure>\n\n")
	fmt.Fprintf(&sb, "Summary: %d domains, %d total behaviors.",
		len(input.Domains), totalBehaviors)

	return sb.String()
}
