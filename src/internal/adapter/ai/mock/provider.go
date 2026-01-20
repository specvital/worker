package mock

import (
	"context"
	"fmt"
	"strings"

	"github.com/specvital/worker/internal/domain/specview"
)

const defaultConfidence = 0.95

// Provider implements specview.AIProvider with deterministic mock responses.
// Intended for local development and testing without AI API calls.
type Provider struct{}

// NewProvider creates a new mock AI provider.
func NewProvider() *Provider {
	return &Provider{}
}

// ClassifyDomains returns mock domain classification based on input test structure.
func (p *Provider) ClassifyDomains(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
	return generatePhase1Output(input), nil, nil
}

// ConvertTestNames returns mock behavior descriptions based on test names.
func (p *Provider) ConvertTestNames(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
	return generatePhase2Output(input), nil, nil
}

// Close releases resources (no-op for mock).
func (p *Provider) Close() error {
	return nil
}

// generatePhase1Output creates mock domain classification from input files.
func generatePhase1Output(input specview.Phase1Input) *specview.Phase1Output {
	if len(input.Files) == 0 {
		return &specview.Phase1Output{Domains: []specview.DomainGroup{}}
	}

	domains := classifyByDirectory(input)

	if len(domains) == 0 {
		domains = createDefaultDomain(input)
	}

	return &specview.Phase1Output{Domains: domains}
}

// classifyByDirectory groups tests by their directory path into domains.
func classifyByDirectory(input specview.Phase1Input) []specview.DomainGroup {
	dirToTests := make(map[string][]testRef)

	for _, file := range input.Files {
		dir := extractDirectory(file.Path)
		for _, test := range file.Tests {
			dirToTests[dir] = append(dirToTests[dir], testRef{
				index: test.Index,
				name:  test.Name,
			})
		}
	}

	var domains []specview.DomainGroup
	for dir, tests := range dirToTests {
		domainName := formatDomainName(dir, input.Language)
		features := groupTestsIntoFeatures(tests, input.Language)

		domains = append(domains, specview.DomainGroup{
			Confidence:  defaultConfidence,
			Description: formatDomainDescription(dir, input.Language),
			Features:    features,
			Name:        domainName,
		})
	}

	return domains
}

// createDefaultDomain creates a single domain containing all tests.
func createDefaultDomain(input specview.Phase1Input) []specview.DomainGroup {
	var allTests []testRef
	for _, file := range input.Files {
		for _, test := range file.Tests {
			allTests = append(allTests, testRef{
				index: test.Index,
				name:  test.Name,
			})
		}
	}

	domainName := "Core Domain"
	domainDesc := "Core functionality and features"
	if input.Language == "Korean" {
		domainName = "핵심 도메인"
		domainDesc = "핵심 기능 및 특성"
	}

	return []specview.DomainGroup{
		{
			Confidence:  defaultConfidence,
			Description: domainDesc,
			Features:    groupTestsIntoFeatures(allTests, input.Language),
			Name:        domainName,
		},
	}
}

// testRef holds test index and name for grouping.
type testRef struct {
	index int
	name  string
}

// groupTestsIntoFeatures groups tests into features (max 2 features per domain).
func groupTestsIntoFeatures(tests []testRef, language specview.Language) []specview.FeatureGroup {
	if len(tests) == 0 {
		return nil
	}

	featureNameA := "Primary Feature"
	featureDescA := "Primary feature tests"
	featureNameB := "Secondary Feature"
	featureDescB := "Secondary feature tests"

	if language == "Korean" {
		featureNameA = "주요 기능"
		featureDescA = "주요 기능 테스트"
		featureNameB = "보조 기능"
		featureDescB = "보조 기능 테스트"
	}

	if len(tests) <= 3 {
		indices := make([]int, len(tests))
		for i, t := range tests {
			indices[i] = t.index
		}
		return []specview.FeatureGroup{
			{
				Confidence:  defaultConfidence,
				Description: featureDescA,
				Name:        featureNameA,
				TestIndices: indices,
			},
		}
	}

	mid := len(tests) / 2
	indicesA := make([]int, mid)
	indicesB := make([]int, len(tests)-mid)

	for i := range mid {
		indicesA[i] = tests[i].index
	}
	for i := mid; i < len(tests); i++ {
		indicesB[i-mid] = tests[i].index
	}

	return []specview.FeatureGroup{
		{
			Confidence:  defaultConfidence,
			Description: featureDescA,
			Name:        featureNameA,
			TestIndices: indicesA,
		},
		{
			Confidence:  0.90,
			Description: featureDescB,
			Name:        featureNameB,
			TestIndices: indicesB,
		},
	}
}

// generatePhase2Output creates mock behavior descriptions from test names.
func generatePhase2Output(input specview.Phase2Input) *specview.Phase2Output {
	behaviors := make([]specview.BehaviorSpec, len(input.Tests))
	for i, test := range input.Tests {
		behaviors[i] = specview.BehaviorSpec{
			Confidence:  defaultConfidence,
			Description: formatBehaviorDescription(test.Name, input.Language),
			TestIndex:   test.Index,
		}
	}
	return &specview.Phase2Output{Behaviors: behaviors}
}

// formatBehaviorDescription converts a test name to a behavior description.
func formatBehaviorDescription(testName string, language specview.Language) string {
	readable := camelCaseToReadable(testName)

	if language == "Korean" {
		return fmt.Sprintf("[Mock] %s 기능을 검증한다", readable)
	}
	return fmt.Sprintf("[Mock] Verifies that %s", readable)
}

// camelCaseToReadable converts CamelCase or snake_case to readable format.
func camelCaseToReadable(s string) string {
	s = strings.TrimPrefix(s, "Test")
	s = strings.TrimPrefix(s, "test_")

	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune(' ')
		}
		if r == '_' {
			result.WriteRune(' ')
		} else {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// extractDirectory extracts the directory portion from a file path.
func extractDirectory(path string) string {
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return "root"
	}
	return path[:lastSlash]
}

// formatDomainName formats a directory path into a domain name.
func formatDomainName(dir string, language specview.Language) string {
	parts := strings.Split(dir, "/")
	name := parts[len(parts)-1]
	if name == "" || name == "." {
		name = "Core"
	}

	name = strings.ReplaceAll(name, "_", " ")
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}

	if language == "Korean" {
		return name + " 도메인"
	}
	return name + " Domain"
}

// formatDomainDescription formats a directory path into a domain description.
func formatDomainDescription(dir string, language specview.Language) string {
	if language == "Korean" {
		return fmt.Sprintf("[Mock] %s 디렉토리의 테스트", dir)
	}
	return fmt.Sprintf("[Mock] Tests from %s directory", dir)
}
