package gemini

import (
	"context"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestParsePhase1Response_ValidJSON(t *testing.T) {
	jsonStr := `{
		"domains": [
			{
				"name": "Authentication",
				"description": "User authentication features",
				"confidence": 0.95,
				"features": [
					{
						"name": "Login",
						"description": "User login functionality",
						"confidence": 0.92,
						"test_indices": [0, 1, 2]
					},
					{
						"name": "Logout",
						"description": "User logout functionality",
						"confidence": 0.88,
						"test_indices": [3, 4]
					}
				]
			}
		]
	}`

	output, err := parsePhase1Response(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(output.Domains))
	}

	domain := output.Domains[0]
	if domain.Name != "Authentication" {
		t.Errorf("expected domain name 'Authentication', got %q", domain.Name)
	}
	if domain.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", domain.Confidence)
	}
	if len(domain.Features) != 2 {
		t.Errorf("expected 2 features, got %d", len(domain.Features))
	}

	feature := domain.Features[0]
	if feature.Name != "Login" {
		t.Errorf("expected feature name 'Login', got %q", feature.Name)
	}
	if len(feature.TestIndices) != 3 {
		t.Errorf("expected 3 test indices, got %d", len(feature.TestIndices))
	}
}

func TestParsePhase1Response_InvalidJSON(t *testing.T) {
	_, err := parsePhase1Response("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParsePhase1Response_EmptyDomains(t *testing.T) {
	jsonStr := `{"domains": []}`

	output, err := parsePhase1Response(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Domains) != 0 {
		t.Errorf("expected 0 domains, got %d", len(output.Domains))
	}
}

func TestValidatePhase1Output_ValidOutput(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "Test1"},
					{Index: 1, Name: "Test2"},
					{Index: 2, Name: "Test3"},
				},
			},
		},
	}

	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:       "Domain1",
				Confidence: 0.9,
				Features: []specview.FeatureGroup{
					{
						Name:        "Feature1",
						Confidence:  0.85,
						TestIndices: []int{0, 1, 2},
					},
				},
			},
		},
	}

	err := validatePhase1Output(context.Background(), output, input)
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestValidatePhase1Output_NilOutput(t *testing.T) {
	input := specview.Phase1Input{}

	err := validatePhase1Output(context.Background(), nil, input)
	if err == nil {
		t.Error("expected error for nil output")
	}
}

func TestValidatePhase1Output_EmptyDomains(t *testing.T) {
	input := specview.Phase1Input{}
	output := &specview.Phase1Output{Domains: []specview.DomainGroup{}}

	err := validatePhase1Output(context.Background(), output, input)
	if err == nil {
		t.Error("expected error for empty domains")
	}
}

func TestValidatePhase1Output_EmptyDomainName(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path:  "test.go",
				Tests: []specview.TestInfo{{Index: 0, Name: "Test1"}},
			},
		},
	}

	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name: "", // Empty name
				Features: []specview.FeatureGroup{
					{Name: "Feature", TestIndices: []int{0}},
				},
			},
		},
	}

	err := validatePhase1Output(context.Background(), output, input)
	if err == nil {
		t.Error("expected error for empty domain name")
	}
}

func TestValidatePhase1Output_EmptyFeatureName(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path:  "test.go",
				Tests: []specview.TestInfo{{Index: 0, Name: "Test1"}},
			},
		},
	}

	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name: "Domain",
				Features: []specview.FeatureGroup{
					{Name: "", TestIndices: []int{0}}, // Empty name
				},
			},
		},
	}

	err := validatePhase1Output(context.Background(), output, input)
	if err == nil {
		t.Error("expected error for empty feature name")
	}
}

func TestValidatePhase1Output_UnexpectedTestIndex(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path:  "test.go",
				Tests: []specview.TestInfo{{Index: 0, Name: "Test1"}},
			},
		},
	}

	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name: "Domain",
				Features: []specview.FeatureGroup{
					{Name: "Feature", TestIndices: []int{0, 999}}, // 999 doesn't exist
				},
			},
		},
	}

	err := validatePhase1Output(context.Background(), output, input)
	// Should NOT error - unexpected indices are filtered out instead of failing
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify that index 999 was filtered out
	if len(output.Domains[0].Features[0].TestIndices) != 1 {
		t.Errorf("expected 1 test index after filtering, got %d", len(output.Domains[0].Features[0].TestIndices))
	}
	if output.Domains[0].Features[0].TestIndices[0] != 0 {
		t.Errorf("expected index 0, got %d", output.Domains[0].Features[0].TestIndices[0])
	}
}

func TestTruncateForLog(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"longer than max", 10, "longer tha..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		got := truncateForLog(tt.input, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncateForLog(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}

func TestValidatePhase1Output_MissingIndicesAutoRecovery(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "Test1"},
					{Index: 1, Name: "Test2"},
					{Index: 2, Name: "Test3"},
					{Index: 3, Name: "Test4"},
					{Index: 4, Name: "Test5"},
				},
			},
		},
	}

	// Output missing indices 2 and 4
	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:       "Domain1",
				Confidence: 0.9,
				Features: []specview.FeatureGroup{
					{
						Name:        "Feature1",
						Confidence:  0.85,
						TestIndices: []int{0, 1, 3}, // Missing 2 and 4
					},
				},
			},
		},
	}

	err := validatePhase1Output(context.Background(), output, input)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	// Should have Uncategorized domain added
	if len(output.Domains) != 2 {
		t.Fatalf("expected 2 domains after auto-recovery, got %d", len(output.Domains))
	}

	uncategorizedDomain := output.Domains[1]
	if uncategorizedDomain.Name != "Uncategorized" {
		t.Errorf("expected Uncategorized domain, got %q", uncategorizedDomain.Name)
	}

	if len(uncategorizedDomain.Features) != 1 {
		t.Fatalf("expected 1 feature in Uncategorized domain, got %d", len(uncategorizedDomain.Features))
	}

	uncategorizedFeature := uncategorizedDomain.Features[0]
	if uncategorizedFeature.Name != "Uncategorized Tests" {
		t.Errorf("expected 'Uncategorized Tests' feature, got %q", uncategorizedFeature.Name)
	}

	// Should contain missing indices 2 and 4
	missingSet := make(map[int]bool)
	for _, idx := range uncategorizedFeature.TestIndices {
		missingSet[idx] = true
	}

	if !missingSet[2] || !missingSet[4] {
		t.Errorf("expected indices 2 and 4 in Uncategorized, got %v", uncategorizedFeature.TestIndices)
	}
}

func TestValidatePhase1Output_NoMissingIndices(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "Test1"},
					{Index: 1, Name: "Test2"},
				},
			},
		},
	}

	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:       "Domain1",
				Confidence: 0.9,
				Features: []specview.FeatureGroup{
					{
						Name:        "Feature1",
						Confidence:  0.85,
						TestIndices: []int{0, 1}, // All indices covered
					},
				},
			},
		},
	}

	err := validatePhase1Output(context.Background(), output, input)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	// Should NOT add Uncategorized domain
	if len(output.Domains) != 1 {
		t.Errorf("expected 1 domain (no Uncategorized added), got %d", len(output.Domains))
	}
}

func TestValidatePhase1Output_ExistingUncategorizedDomain(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "Test1"},
					{Index: 1, Name: "Test2"},
					{Index: 2, Name: "Test3"},
				},
			},
		},
	}

	// Output already has Uncategorized domain from previous recovery
	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:       "Domain1",
				Confidence: 0.9,
				Features: []specview.FeatureGroup{
					{
						Name:        "Feature1",
						Confidence:  0.85,
						TestIndices: []int{0}, // Missing 1 and 2
					},
				},
			},
			{
				Name:        "Uncategorized",
				Description: "Tests that could not be classified by AI",
				Confidence:  0.5,
				Features: []specview.FeatureGroup{
					{
						Name:        "Uncategorized Tests",
						Description: "Tests that could not be classified by AI",
						Confidence:  0.5,
						TestIndices: []int{}, // Will be appended to
					},
				},
			},
		},
	}

	err := validatePhase1Output(context.Background(), output, input)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	// Should still have 2 domains
	if len(output.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(output.Domains))
	}

	// Missing indices should be appended to existing Uncategorized feature
	uncategorizedDomain := output.Domains[1]
	if len(uncategorizedDomain.Features[0].TestIndices) != 2 {
		t.Errorf("expected 2 missing indices appended, got %d", len(uncategorizedDomain.Features[0].TestIndices))
	}
}

func TestAddUncategorizedDomain_NewDomain(t *testing.T) {
	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{Name: "Domain1", Features: []specview.FeatureGroup{{Name: "F1"}}},
		},
	}

	addUncategorizedDomain(output, []int{5, 10, 15})

	if len(output.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(output.Domains))
	}

	uncategorized := output.Domains[1]
	if uncategorized.Name != "Uncategorized" {
		t.Errorf("expected Uncategorized domain, got %q", uncategorized.Name)
	}
	if uncategorized.Confidence != 0.5 {
		t.Errorf("expected confidence 0.5, got %f", uncategorized.Confidence)
	}
	if len(uncategorized.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(uncategorized.Features))
	}
	if len(uncategorized.Features[0].TestIndices) != 3 {
		t.Errorf("expected 3 indices, got %d", len(uncategorized.Features[0].TestIndices))
	}
}

func TestAddUncategorizedDomain_ExistingFeature(t *testing.T) {
	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{Name: "Domain1", Features: []specview.FeatureGroup{{Name: "F1"}}},
			{
				Name: "Uncategorized",
				Features: []specview.FeatureGroup{
					{Name: "Uncategorized Tests", TestIndices: []int{1, 2}},
				},
			},
		},
	}

	addUncategorizedDomain(output, []int{5, 10})

	if len(output.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(output.Domains))
	}

	uncategorized := output.Domains[1]
	if len(uncategorized.Features[0].TestIndices) != 4 {
		t.Errorf("expected 4 indices (2 original + 2 new), got %d", len(uncategorized.Features[0].TestIndices))
	}
}

func TestValidatePhase1Output_OffByOneIndex(t *testing.T) {
	// Simulates the case where AI returns index N for N tests (0-indexed should be 0 to N-1)
	// This is the root cause of the chunk 8 failure with 248 tests
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.go",
				Tests: func() []specview.TestInfo {
					tests := make([]specview.TestInfo, 248)
					for i := 0; i < 248; i++ {
						tests[i] = specview.TestInfo{Index: i, Name: "Test"}
					}
					return tests
				}(),
			},
		},
	}

	// AI returns index 248 (off-by-one error) along with some valid indices
	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name:       "Domain",
				Confidence: 0.9,
				Features: []specview.FeatureGroup{
					{
						Name:        "Feature1",
						Confidence:  0.85,
						TestIndices: []int{0, 1, 2, 247, 248}, // 248 is invalid (off-by-one)
					},
				},
			},
		},
	}

	err := validatePhase1Output(context.Background(), output, input)
	// Should NOT error - index 248 should be filtered out
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that index 248 was filtered out
	feature := output.Domains[0].Features[0]
	for _, idx := range feature.TestIndices {
		if idx == 248 {
			t.Error("index 248 should have been filtered out")
		}
	}

	// Valid indices should remain
	expectedIndices := map[int]bool{0: true, 1: true, 2: true, 247: true}
	if len(feature.TestIndices) != 4 {
		t.Errorf("expected 4 valid indices, got %d", len(feature.TestIndices))
	}
	for _, idx := range feature.TestIndices {
		if !expectedIndices[idx] {
			t.Errorf("unexpected index %d in filtered output", idx)
		}
	}
}

func TestValidatePhase1Output_MultipleUnexpectedIndices(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "Test1"},
					{Index: 1, Name: "Test2"},
					{Index: 2, Name: "Test3"},
				},
			},
		},
	}

	// Multiple unexpected indices scattered across features
	output := &specview.Phase1Output{
		Domains: []specview.DomainGroup{
			{
				Name: "Domain1",
				Features: []specview.FeatureGroup{
					{Name: "Feature1", TestIndices: []int{0, 100}},   // 100 is invalid
					{Name: "Feature2", TestIndices: []int{1, -1, 2}}, // -1 is invalid
				},
			},
			{
				Name: "Domain2",
				Features: []specview.FeatureGroup{
					{Name: "Feature3", TestIndices: []int{999}}, // All invalid
				},
			},
		},
	}

	err := validatePhase1Output(context.Background(), output, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check Feature1: should only have index 0
	if len(output.Domains[0].Features[0].TestIndices) != 1 {
		t.Errorf("Feature1: expected 1 index, got %d", len(output.Domains[0].Features[0].TestIndices))
	}

	// Check Feature2: should have indices 1 and 2
	if len(output.Domains[0].Features[1].TestIndices) != 2 {
		t.Errorf("Feature2: expected 2 indices, got %d", len(output.Domains[0].Features[1].TestIndices))
	}

	// Check Feature3: should be empty
	if len(output.Domains[1].Features[0].TestIndices) != 0 {
		t.Errorf("Feature3: expected 0 indices, got %d", len(output.Domains[1].Features[0].TestIndices))
	}
}

