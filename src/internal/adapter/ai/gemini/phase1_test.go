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
	if err == nil {
		t.Error("expected error for unexpected test index")
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

func TestClassifyDomains_EmptyFiles_ReturnsError(t *testing.T) {
	p := &Provider{
		phase1Model: "test-model",
	}

	_, _, err := p.classifyDomains(context.Background(), specview.Phase1Input{}, "Korean")
	if err == nil {
		t.Error("expected error for empty files")
	}
}

