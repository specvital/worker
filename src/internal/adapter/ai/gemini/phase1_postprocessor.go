package gemini

import (
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/specvital/worker/internal/domain/specview"
)

// domainAbbreviations maps short forms to their canonical expanded forms.
var domainAbbreviations = map[string]string{
	"auth":       "authentication",
	"authn":      "authentication",
	"authz":      "authorization",
	"config":     "configuration",
	"db":         "database",
	"doc":        "documentation",
	"docs":       "documentation",
	"err":        "error handling",
	"errors":     "error handling",
	"nav":        "navigation",
	"perf":       "performance",
	"sec":        "security",
	"test":       "testing",
	"tests":      "testing",
	"ui":         "user interface",
	"util":       "utilities",
	"utils":      "utilities",
	"ux":         "user experience",
	"valid":      "validation",
	"validation": "validation",
}

// pathSkipPrefixes are directory names that don't contribute to domain meaning.
var pathSkipPrefixes = []string{"src", "test", "tests", "spec", "specs", "__tests__", "__test__", "lib", "packages"}

// PostProcessorConfig configures the Phase1PostProcessor behavior.
type PostProcessorConfig struct {
	// ProhibitUncategorized rejects results containing Uncategorized/General.
	ProhibitUncategorized bool
}

// DefaultPostProcessorConfig returns the default configuration.
func DefaultPostProcessorConfig() PostProcessorConfig {
	return PostProcessorConfig{
		ProhibitUncategorized: true,
	}
}

// QualityViolationType identifies the type of quality violation.
type QualityViolationType string

const (
	ViolationOrphanedTest  QualityViolationType = "orphaned_test"
	ViolationUncategorized QualityViolationType = "uncategorized"
)

// QualityViolation represents a single quality violation in classification results.
type QualityViolation struct {
	Type    QualityViolationType
	Details string
}

// Phase1PostProcessor validates and normalizes Phase 1 classification results.
type Phase1PostProcessor struct {
	config PostProcessorConfig
}

// NewPhase1PostProcessor creates a new post-processor with the given configuration.
func NewPhase1PostProcessor(config PostProcessorConfig) *Phase1PostProcessor {
	return &Phase1PostProcessor{config: config}
}

// Process validates and normalizes the classification results.
// Returns processed results and any violations found.
func (p *Phase1PostProcessor) Process(
	results []v3BatchResult,
	tests []specview.TestForAssignment,
) ([]v3BatchResult, []QualityViolation) {
	var violations []QualityViolation

	// Step 1: Validate no uncategorized (if configured)
	if p.config.ProhibitUncategorized {
		uncatViolations := validateNoUncategorized(results)
		violations = append(violations, uncatViolations...)
	}

	// Step 2: Find orphaned tests
	orphanViolations := findOrphanedTests(results, tests)
	violations = append(violations, orphanViolations...)

	// Step 3: Normalize domains (merge similar names)
	normalizedResults := normalizeDomains(results)

	// Step 4: Replace uncategorized with path-derived domains (if any remain)
	finalResults := replaceUncategorizedWithPathDomains(normalizedResults, tests)

	return finalResults, violations
}

// validateNoUncategorized checks for prohibited Uncategorized/General values.
func validateNoUncategorized(results []v3BatchResult) []QualityViolation {
	var violations []QualityViolation

	for i, r := range results {
		if isUncategorized(r.Domain, r.Feature) {
			violations = append(violations, QualityViolation{
				Type:    ViolationUncategorized,
				Details: formatViolationDetails(i, r.Domain, r.Feature),
			})
		}
	}

	return violations
}

// formatViolationDetails creates a human-readable violation description.
func formatViolationDetails(index int, domain, feature string) string {
	return "index " + strconv.Itoa(index) + ": " + domain + "/" + feature
}

// findOrphanedTests detects tests that are not assigned in results.
func findOrphanedTests(results []v3BatchResult, tests []specview.TestForAssignment) []QualityViolation {
	var violations []QualityViolation

	if len(results) != len(tests) {
		violations = append(violations, QualityViolation{
			Type:    ViolationOrphanedTest,
			Details: "result count mismatch: expected " + strconv.Itoa(len(tests)) + ", got " + strconv.Itoa(len(results)),
		})
	}

	return violations
}

// normalizeDomains merges similar domain names.
// Example: "Auth", "Authentication", "auth" -> "Authentication"
func normalizeDomains(results []v3BatchResult) []v3BatchResult {
	if len(results) == 0 {
		return results
	}

	// Build domain name mapping
	domainMapping := buildDomainNormalizationMap(results)

	// Apply normalization
	normalized := make([]v3BatchResult, len(results))
	for i, r := range results {
		normalized[i] = v3BatchResult{
			Domain:     domainMapping[r.Domain],
			DomainDesc: r.DomainDesc,
			Feature:    r.Feature,
		}
	}

	return normalized
}

// buildDomainNormalizationMap creates a mapping from domain variants to canonical names.
func buildDomainNormalizationMap(results []v3BatchResult) map[string]string {
	// Collect all unique domains
	domains := make(map[string]struct{})
	for _, r := range results {
		domains[r.Domain] = struct{}{}
	}

	// Sort domains by length descending (longer = more specific = preferred)
	// Use alphabetical order as secondary sort for determinism
	sortedDomains := make([]string, 0, len(domains))
	for d := range domains {
		sortedDomains = append(sortedDomains, d)
	}
	sort.Slice(sortedDomains, func(i, j int) bool {
		if len(sortedDomains[i]) != len(sortedDomains[j]) {
			return len(sortedDomains[i]) > len(sortedDomains[j])
		}
		return sortedDomains[i] < sortedDomains[j]
	})

	// Build normalization groups
	mapping := make(map[string]string)
	for _, domain := range sortedDomains {
		normalized := normalizeDomainName(domain)

		// Check if this normalized form already has a canonical name
		canonical := findCanonicalDomain(mapping, normalized)
		if canonical == "" {
			canonical = domain
		}

		mapping[domain] = canonical
	}

	return mapping
}

// findCanonicalDomain finds an existing canonical domain for a normalized name.
func findCanonicalDomain(mapping map[string]string, normalizedTarget string) string {
	for original, canonical := range mapping {
		if areSimilarDomains(normalizeDomainName(original), normalizedTarget) {
			return canonical
		}
	}
	return ""
}

// areSimilarDomains checks if two normalized domain names should be merged.
func areSimilarDomains(a, b string) bool {
	if a == b {
		return true
	}

	expandedA := expandAbbreviation(a)
	expandedB := expandAbbreviation(b)

	return expandedA == expandedB
}

// expandAbbreviation expands a known abbreviation to its full form.
func expandAbbreviation(s string) string {
	if expanded, ok := domainAbbreviations[s]; ok {
		return expanded
	}
	return s
}

// replaceUncategorizedWithPathDomains replaces uncategorized results with path-derived domains.
func replaceUncategorizedWithPathDomains(results []v3BatchResult, tests []specview.TestForAssignment) []v3BatchResult {
	if len(results) != len(tests) {
		return results
	}

	replaced := make([]v3BatchResult, len(results))
	for i, r := range results {
		if isUncategorized(r.Domain, r.Feature) {
			pathDomain, pathFeature := deriveDomainFromPath(tests[i].FilePath)
			replaced[i] = v3BatchResult{
				Domain:     pathDomain,
				DomainDesc: "Derived from file path",
				Feature:    pathFeature,
			}
		} else {
			replaced[i] = r
		}
	}

	return replaced
}

// deriveDomainFromPath extracts a domain and feature name from a file path.
// Uses directory structure to infer meaningful categorization.
func deriveDomainFromPath(filePath string) (domain, feature string) {
	if filePath == "" {
		return "Project Root", "General Tests"
	}

	// Clean and split path
	cleanPath := strings.TrimPrefix(filePath, "/")
	cleanPath = strings.TrimPrefix(cleanPath, "./")

	dir := path.Dir(cleanPath)
	if dir == "." || dir == "" {
		return "Project Root", "General Tests"
	}

	parts := strings.Split(dir, "/")
	parts = filterEmptyParts(parts)

	if len(parts) == 0 {
		return "Project Root", "General Tests"
	}

	// Filter meaningful parts
	significantParts := filterSignificantParts(parts)

	// Extract meaningful domain and feature from path
	domain = extractDomainFromSignificantParts(significantParts, parts)
	feature = extractFeatureFromSignificantParts(significantParts)

	return domain, feature
}

// filterEmptyParts removes empty strings from path parts.
func filterEmptyParts(parts []string) []string {
	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// filterSignificantParts filters out common prefixes that don't add meaning.
func filterSignificantParts(parts []string) []string {
	significantParts := make([]string, 0, len(parts))
	for _, p := range parts {
		lower := strings.ToLower(p)
		if !slices.Contains(pathSkipPrefixes, lower) {
			significantParts = append(significantParts, p)
		}
	}

	return significantParts
}

// extractDomainFromSignificantParts creates a domain name from filtered path parts.
func extractDomainFromSignificantParts(significantParts, allParts []string) string {
	if len(significantParts) == 0 {
		if len(allParts) > 0 {
			return formatDomainName(allParts[len(allParts)-1])
		}
		return "Project Root"
	}

	// Use first significant part as domain
	return formatDomainName(significantParts[0])
}

// extractFeatureFromSignificantParts creates a feature name from filtered path parts.
func extractFeatureFromSignificantParts(significantParts []string) string {
	if len(significantParts) <= 1 {
		return "Core"
	}

	// Use last significant part as feature (often the most specific)
	return formatFeatureName(significantParts[len(significantParts)-1])
}

// formatDomainName formats a path segment into a proper domain name.
func formatDomainName(segment string) string {
	// Convert kebab-case, snake_case to Title Case
	segment = strings.ReplaceAll(segment, "-", " ")
	segment = strings.ReplaceAll(segment, "_", " ")

	words := strings.Fields(segment)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}

	if len(words) == 0 {
		return "Unknown"
	}

	return strings.Join(words, " ")
}

// formatFeatureName formats a path segment into a proper feature name.
func formatFeatureName(segment string) string {
	return formatDomainName(segment)
}

// createDomainsFromPaths creates domain classifications based purely on file paths.
// Used as a fallback when AI classification fails completely.
func createDomainsFromPaths(tests []specview.TestForAssignment) []v3BatchResult {
	results := make([]v3BatchResult, len(tests))

	for i, test := range tests {
		domain, feature := deriveDomainFromPath(test.FilePath)
		results[i] = v3BatchResult{
			Domain:     domain,
			DomainDesc: "Derived from file path",
			Feature:    feature,
		}
	}

	return results
}
