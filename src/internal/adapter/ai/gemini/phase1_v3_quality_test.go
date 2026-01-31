package gemini

import (
	"context"
	"fmt"
	"testing"

	"github.com/specvital/worker/internal/adapter/ai/prompt"
	"github.com/specvital/worker/internal/domain/specview"
)

// --- Quality Integration Tests ---
// These tests verify that the V3 classification pipeline eliminates Uncategorized results.

func TestV3Quality_ZeroUncategorizedRate(t *testing.T) {
	// Verify that 100 diverse tests result in 0% Uncategorized rate
	input := createDiverseTestInput(100)

	provider := newMockV3Provider(func(ctx context.Context, tests []specview.TestForAssignment, existingDomains []prompt.DomainSummary) ([]v3BatchResult, *specview.TokenUsage, error) {
		results := make([]v3BatchResult, len(tests))
		for i, test := range tests {
			domain, feature := classifyTestByPath(test.FilePath)
			results[i] = v3BatchResult{
				Domain:     domain,
				DomainDesc: "Test classification",
				Feature:    feature,
			}
		}
		return results, &specview.TokenUsage{PromptTokens: 100}, nil
	})

	ctx := context.Background()
	output, _, err := provider.classifyDomainsV3Integration(ctx, input, "Korean")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no Uncategorized domain exists
	for _, domain := range output.Domains {
		if isUncategorized(domain.Name, "") {
			t.Errorf("found Uncategorized domain: %q", domain.Name)
		}
		for _, feature := range domain.Features {
			if isUncategorized("", feature.Name) {
				t.Errorf("found Uncategorized feature in domain %q: %q", domain.Name, feature.Name)
			}
		}
	}

	// Verify all 100 tests are classified
	totalTests := countTotalTests(output)
	if totalTests != 100 {
		t.Errorf("expected 100 tests classified, got %d", totalTests)
	}
}

func TestV3Quality_AIReturnsUncategorized_TransformedByPostProcessor(t *testing.T) {
	// AI returns Uncategorized -> post-processor should transform to path-based domain
	input := createV3TestInput(2, 3) // 6 tests
	tests := flattenTests(input.Files)

	// Simulate AI returning all Uncategorized
	rawResults := make([]v3BatchResult, len(tests))
	for i := range tests {
		rawResults[i] = v3BatchResult{
			Domain:  "Uncategorized",
			Feature: "General",
		}
	}

	// Apply post-processing
	pp := NewPhase1PostProcessor(DefaultPostProcessorConfig())
	processed, violations := pp.Process(rawResults, tests)

	// Should have violations for all Uncategorized results
	if len(violations) != 6 {
		t.Errorf("expected 6 violations, got %d", len(violations))
	}

	// After processing, no Uncategorized should remain
	for _, r := range processed {
		if r.Domain == "Uncategorized" {
			t.Error("Uncategorized domain should have been transformed")
		}
		if r.Feature == "General" && r.Domain != "Project Root" {
			t.Errorf("General feature should have been transformed: %+v", r)
		}
	}
}

func TestV3Quality_MetricsShowZeroUncategorizedRate(t *testing.T) {
	// Verify metrics collector properly tracks 0% Uncategorized rate
	collector := NewPhase1MetricsCollector()

	// Record batches with proper classifications (no Uncategorized)
	batch1 := []v3BatchResult{
		{Domain: "Authentication", Feature: "Login"},
		{Domain: "Authentication", Feature: "Logout"},
		{Domain: "Navigation", Feature: "Routing"},
	}
	collector.RecordBatch(batch1, 0, 0)

	batch2 := []v3BatchResult{
		{Domain: "Forms", Feature: "Validation"},
		{Domain: "Forms", Feature: "Submission"},
	}
	collector.RecordBatch(batch2, 0, 0)

	metrics := collector.Collect()

	if metrics.UncategorizedRate != 0 {
		t.Errorf("expected UncategorizedRate=0, got %f", metrics.UncategorizedRate)
	}
	if metrics.TotalTests != 5 {
		t.Errorf("expected TotalTests=5, got %d", metrics.TotalTests)
	}
	if metrics.ClassificationRate != 1.0 {
		t.Errorf("expected ClassificationRate=1.0, got %f", metrics.ClassificationRate)
	}
}

func TestV3Quality_DomainNormalizationReducesDomainCount(t *testing.T) {
	// Verify domain normalization merges similar domains
	results := []v3BatchResult{
		{Domain: "Authentication", Feature: "Login"},
		{Domain: "Auth", Feature: "Logout"},
		{Domain: "auth", Feature: "Session"},
		{Domain: "Navigation", Feature: "Routing"},
		{Domain: "Nav", Feature: "Links"},
	}
	tests := make([]specview.TestForAssignment, len(results))
	for i := range tests {
		tests[i] = specview.TestForAssignment{
			FilePath: fmt.Sprintf("src/test%d.ts", i),
			Index:    i,
		}
	}

	pp := NewPhase1PostProcessor(DefaultPostProcessorConfig())
	processed, _ := pp.Process(results, tests)

	// Count unique domains after normalization
	uniqueDomains := make(map[string]bool)
	for _, r := range processed {
		uniqueDomains[r.Domain] = true
	}

	// Should have 2 domains (Authentication, Navigation) instead of 5
	if len(uniqueDomains) != 2 {
		t.Errorf("expected 2 unique domains after normalization, got %d: %v",
			len(uniqueDomains), uniqueDomains)
	}

	// Verify Authentication is the canonical name (not Auth or auth)
	if !uniqueDomains["Authentication"] {
		t.Error("expected 'Authentication' as canonical domain name")
	}
	if !uniqueDomains["Navigation"] {
		t.Error("expected 'Navigation' as canonical domain name")
	}
}

func TestV3Quality_PathBasedFallbackNeverProducesUncategorized(t *testing.T) {
	// Verify path-based fallback never produces Uncategorized domains
	testPaths := []string{
		"src/auth/login.test.ts",
		"src/components/Button.test.tsx",
		"packages/core/api/client.test.ts",
		"tests/integration/auth-flow.test.ts",
		"__tests__/utils/helpers.test.ts",
		"lib/database/connection.test.ts",
		"test.spec.ts",
		"",
	}

	for _, path := range testPaths {
		domain, feature := deriveDomainFromPath(path)

		if isUncategorized(domain, feature) {
			t.Errorf("deriveDomainFromPath(%q) produced Uncategorized: domain=%q, feature=%q",
				path, domain, feature)
		}
	}
}

func TestV3Quality_CompleteFlowWithPostProcessing(t *testing.T) {
	// End-to-end test: batch processing -> metrics collection -> post-processing
	input := createDiverseTestInput(50)

	var recordedBatches int
	provider := newMockV3Provider(func(ctx context.Context, tests []specview.TestForAssignment, existingDomains []prompt.DomainSummary) ([]v3BatchResult, *specview.TokenUsage, error) {
		recordedBatches++
		results := make([]v3BatchResult, len(tests))
		for i, test := range tests {
			domain, feature := classifyTestByPath(test.FilePath)
			results[i] = v3BatchResult{
				Domain:     domain,
				DomainDesc: "AI classification",
				Feature:    feature,
			}
		}
		return results, &specview.TokenUsage{PromptTokens: 100}, nil
	})

	ctx := context.Background()
	output, _, err := provider.classifyDomainsV3Integration(ctx, input, "Korean")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify batching occurred (50 tests / 20 per batch = 3 batches)
	if recordedBatches != 3 {
		t.Errorf("expected 3 batches, got %d", recordedBatches)
	}

	// Verify all tests classified
	totalTests := countTotalTests(output)
	if totalTests != 50 {
		t.Errorf("expected 50 tests, got %d", totalTests)
	}

	// Verify no Uncategorized
	for _, domain := range output.Domains {
		if isUncategorized(domain.Name, "") {
			t.Errorf("found Uncategorized domain: %q", domain.Name)
		}
	}
}

func TestV3Quality_MixedResultsWithSomeUncategorized(t *testing.T) {
	// Some AI results are Uncategorized, should be transformed by post-processor
	input := createDiverseTestInput(10)
	tests := flattenTests(input.Files)

	// Simulate AI returning mixed results
	rawResults := make([]v3BatchResult, len(tests))
	for i, test := range tests {
		if i%3 == 0 {
			// Every 3rd result is Uncategorized
			rawResults[i] = v3BatchResult{
				Domain:  "Uncategorized",
				Feature: "General",
			}
		} else {
			domain, feature := classifyTestByPath(test.FilePath)
			rawResults[i] = v3BatchResult{
				Domain:  domain,
				Feature: feature,
			}
		}
	}

	// Apply post-processing
	pp := NewPhase1PostProcessor(DefaultPostProcessorConfig())
	processed, violations := pp.Process(rawResults, tests)

	// Should have violations for Uncategorized results (indices 0, 3, 6, 9 = 4 violations)
	if len(violations) != 4 {
		t.Errorf("expected 4 violations, got %d", len(violations))
	}

	// No Uncategorized should remain after post-processing
	for i, r := range processed {
		if r.Domain == "Uncategorized" {
			t.Errorf("result[%d]: Uncategorized domain should have been transformed", i)
		}
	}
}

// --- Helper Functions ---

// createDiverseTestInput creates test input with diverse file paths.
func createDiverseTestInput(testCount int) specview.Phase1Input {
	paths := []string{
		"src/auth/login.test.ts",
		"src/auth/logout.test.ts",
		"src/components/Button.test.tsx",
		"src/components/Modal.test.tsx",
		"src/navigation/router.test.ts",
		"src/forms/validation.test.ts",
		"src/api/client.test.ts",
		"src/database/connection.test.ts",
		"src/utils/helpers.test.ts",
		"src/config/settings.test.ts",
	}

	files := make([]specview.FileInfo, 0)
	testIndex := 0

	for testIndex < testCount {
		pathIdx := testIndex % len(paths)
		path := paths[pathIdx]

		testsInFile := min(5, testCount-testIndex)
		tests := make([]specview.TestInfo, testsInFile)
		for j := range testsInFile {
			tests[j] = specview.TestInfo{
				Index: testIndex,
				Name:  fmt.Sprintf("test_%d", testIndex),
			}
			testIndex++
		}

		files = append(files, specview.FileInfo{
			Path:  path,
			Tests: tests,
		})
	}

	return specview.Phase1Input{
		AnalysisID: "quality-test-id",
		Files:      files,
		Language:   "Korean",
	}
}

// classifyTestByPath returns domain and feature based on file path.
// Simulates proper AI classification without Uncategorized.
func classifyTestByPath(path string) (string, string) {
	domain, feature := deriveDomainFromPath(path)
	return domain, feature
}

// countTotalTests counts all test indices across domains.
func countTotalTests(output *specview.Phase1Output) int {
	total := 0
	for _, domain := range output.Domains {
		for _, feature := range domain.Features {
			total += len(feature.TestIndices)
		}
	}
	return total
}
