package gemini

import (
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestPrepareTaxonomyInput(t *testing.T) {
	input := specview.Phase1Input{
		AnalysisID: "analysis-123",
		Files: []specview.FileInfo{
			{
				Path: "src/auth/login_test.go",
				DomainHints: &specview.DomainHints{
					Imports: []string{"jwt", "bcrypt"},
					Calls:   []string{"loginUser", "validateToken"},
				},
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin"},
					{Index: 1, Name: "TestLogout"},
				},
			},
			{
				Path: "src/payment/stripe_test.go",
				Tests: []specview.TestInfo{
					{Index: 2, Name: "TestCharge"},
				},
			},
		},
		Language: "Korean",
	}

	result := prepareTaxonomyInput(input)

	if result.AnalysisID != "analysis-123" {
		t.Errorf("expected AnalysisID 'analysis-123', got %q", result.AnalysisID)
	}

	if result.Language != "Korean" {
		t.Errorf("expected Language 'Korean', got %q", result.Language)
	}

	if len(result.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result.Files))
	}

	file0 := result.Files[0]
	if file0.Index != 0 {
		t.Errorf("expected file[0].Index = 0, got %d", file0.Index)
	}
	if file0.Path != "src/auth/login_test.go" {
		t.Errorf("expected file[0].Path 'src/auth/login_test.go', got %q", file0.Path)
	}
	if file0.TestCount != 2 {
		t.Errorf("expected file[0].TestCount = 2, got %d", file0.TestCount)
	}
	if file0.DomainHints == nil {
		t.Error("expected file[0].DomainHints to be non-nil")
	}

	file1 := result.Files[1]
	if file1.Index != 1 {
		t.Errorf("expected file[1].Index = 1, got %d", file1.Index)
	}
	if file1.TestCount != 1 {
		t.Errorf("expected file[1].TestCount = 1, got %d", file1.TestCount)
	}
	if file1.DomainHints != nil {
		t.Error("expected file[1].DomainHints to be nil")
	}
}

func TestPrepareTaxonomyInput_EmptyFiles(t *testing.T) {
	input := specview.Phase1Input{
		AnalysisID: "analysis-123",
		Files:      []specview.FileInfo{},
		Language:   "English",
	}

	result := prepareTaxonomyInput(input)

	if len(result.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(result.Files))
	}
	if result.AnalysisID != "analysis-123" {
		t.Errorf("expected AnalysisID 'analysis-123', got %q", result.AnalysisID)
	}
}

func TestParseTaxonomyResponse_ValidJSON(t *testing.T) {
	jsonStr := `{
		"domains": [
			{
				"name": "Authentication",
				"description": "User authentication features",
				"features": [
					{"name": "Login", "file_indices": [0, 1]},
					{"name": "Logout", "file_indices": [2]}
				]
			},
			{
				"name": "Payment",
				"description": "Payment processing",
				"features": [
					{"name": "Stripe", "file_indices": [3, 4, 5]}
				]
			}
		]
	}`

	output, err := parseTaxonomyResponse(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(output.Domains))
	}

	domain0 := output.Domains[0]
	if domain0.Name != "Authentication" {
		t.Errorf("expected domain[0].Name 'Authentication', got %q", domain0.Name)
	}
	if domain0.Description != "User authentication features" {
		t.Errorf("expected domain[0].Description 'User authentication features', got %q", domain0.Description)
	}
	if len(domain0.Features) != 2 {
		t.Errorf("expected 2 features in domain[0], got %d", len(domain0.Features))
	}

	feature0 := domain0.Features[0]
	if feature0.Name != "Login" {
		t.Errorf("expected feature[0].Name 'Login', got %q", feature0.Name)
	}
	if len(feature0.FileIndices) != 2 {
		t.Errorf("expected 2 file indices in feature[0], got %d", len(feature0.FileIndices))
	}
}

func TestParseTaxonomyResponse_InvalidJSON(t *testing.T) {
	_, err := parseTaxonomyResponse("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseTaxonomyResponse_EmptyDomains(t *testing.T) {
	_, err := parseTaxonomyResponse(`{"domains": []}`)
	if err == nil {
		t.Error("expected error for empty domains")
	}
}

func TestParseTaxonomyResponse_EmptyDomainName(t *testing.T) {
	jsonStr := `{
		"domains": [
			{
				"name": "",
				"features": [{"name": "Login", "file_indices": [0]}]
			}
		]
	}`

	_, err := parseTaxonomyResponse(jsonStr)
	if err == nil {
		t.Error("expected error for empty domain name")
	}
}

func TestParseTaxonomyResponse_EmptyFeatureName(t *testing.T) {
	jsonStr := `{
		"domains": [
			{
				"name": "Auth",
				"features": [{"name": "", "file_indices": [0]}]
			}
		]
	}`

	_, err := parseTaxonomyResponse(jsonStr)
	if err == nil {
		t.Error("expected error for empty feature name")
	}
}

func TestValidateTaxonomy_AllFilesCovered(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0, 1}},
				},
			},
			{
				Name: "Payment",
				Features: []specview.TaxonomyFeature{
					{Name: "Stripe", FileIndices: []int{2}},
				},
			},
		},
	}

	err := validateTaxonomy(output, 3)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTaxonomy_DuplicateFileIndicesAllowed(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0, 1}},
					{Name: "Session", FileIndices: []int{0}},
				},
			},
			{
				Name: "Security",
				Features: []specview.TaxonomyFeature{
					{Name: "Token", FileIndices: []int{1, 2}},
				},
			},
		},
	}

	err := validateTaxonomy(output, 3)
	if err != nil {
		t.Errorf("unexpected error for duplicate file indices: %v", err)
	}
}

func TestValidateTaxonomy_MissingFiles(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0}},
				},
			},
		},
	}

	err := validateTaxonomy(output, 3)
	if err == nil {
		t.Error("expected error for missing files")
	}
}

func TestValidateTaxonomy_IndexOutOfRange(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0, 99}},
				},
			},
		},
	}

	err := validateTaxonomy(output, 3)
	if err == nil {
		t.Error("expected error for index out of range")
	}
}

func TestValidateTaxonomy_NilOutput(t *testing.T) {
	err := validateTaxonomy(nil, 3)
	if err == nil {
		t.Error("expected error for nil output")
	}
}

func TestValidateTaxonomy_NegativeIndex(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0, -1}},
				},
			},
		},
	}

	err := validateTaxonomy(output, 3)
	if err == nil {
		t.Error("expected error for negative index")
	}
}

func TestRecoverMissingFiles_NoMissingFiles(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0, 1, 2}},
				},
			},
		},
	}

	result := recoverMissingFiles(output, 3)

	if len(result.Domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(result.Domains))
	}
}

func TestRecoverMissingFiles_AddsMissingToNewUncategorized(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0}},
				},
			},
		},
	}

	result := recoverMissingFiles(output, 3)

	if len(result.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(result.Domains))
	}

	uncategorized := result.Domains[1]
	if uncategorized.Name != uncategorizedDomainName {
		t.Errorf("expected domain name %q, got %q", uncategorizedDomainName, uncategorized.Name)
	}
	if len(uncategorized.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(uncategorized.Features))
	}

	general := uncategorized.Features[0]
	if general.Name != uncategorizedFeatureName {
		t.Errorf("expected feature name %q, got %q", uncategorizedFeatureName, general.Name)
	}
	if len(general.FileIndices) != 2 {
		t.Errorf("expected 2 file indices, got %d", len(general.FileIndices))
	}
	if general.FileIndices[0] != 1 || general.FileIndices[1] != 2 {
		t.Errorf("expected file indices [1, 2], got %v", general.FileIndices)
	}
}

func TestRecoverMissingFiles_AppendsToExistingUncategorized(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0}},
				},
			},
			{
				Name:        uncategorizedDomainName,
				Description: "Existing uncategorized",
				Features: []specview.TaxonomyFeature{
					{Name: uncategorizedFeatureName, FileIndices: []int{1}},
				},
			},
		},
	}

	result := recoverMissingFiles(output, 4)

	if len(result.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(result.Domains))
	}

	uncategorized := result.Domains[1]
	if len(uncategorized.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(uncategorized.Features))
	}

	general := uncategorized.Features[0]
	if len(general.FileIndices) != 3 {
		t.Errorf("expected 3 file indices (1 + 2 missing), got %d", len(general.FileIndices))
	}
}

func TestRecoverMissingFiles_CreatesGeneralInExistingUncategorized(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: uncategorizedDomainName,
				Features: []specview.TaxonomyFeature{
					{Name: "OtherFeature", FileIndices: []int{0}},
				},
			},
		},
	}

	result := recoverMissingFiles(output, 3)

	if len(result.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(result.Domains))
	}

	uncategorized := result.Domains[0]
	if len(uncategorized.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(uncategorized.Features))
	}

	general := uncategorized.Features[1]
	if general.Name != uncategorizedFeatureName {
		t.Errorf("expected feature name %q, got %q", uncategorizedFeatureName, general.Name)
	}
	if len(general.FileIndices) != 2 {
		t.Errorf("expected 2 file indices, got %d", len(general.FileIndices))
	}
}

func TestRecoverMissingFiles_SkipsOutOfRangeIndices(t *testing.T) {
	output := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0, -1, 99}},
				},
			},
		},
	}

	result := recoverMissingFiles(output, 3)

	if len(result.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(result.Domains))
	}

	uncategorized := result.Domains[1]
	general := uncategorized.Features[0]
	if len(general.FileIndices) != 2 {
		t.Errorf("expected 2 file indices (1 and 2), got %v", general.FileIndices)
	}
}

func TestCollectCoveredIndices(t *testing.T) {
	tests := []struct {
		name      string
		output    *specview.TaxonomyOutput
		fileCount int
		want      map[int]bool
	}{
		{
			name: "all valid indices",
			output: &specview.TaxonomyOutput{
				Domains: []specview.TaxonomyDomain{
					{
						Name: "Auth",
						Features: []specview.TaxonomyFeature{
							{Name: "Login", FileIndices: []int{0, 1, 2}},
						},
					},
				},
			},
			fileCount: 3,
			want:      map[int]bool{0: true, 1: true, 2: true},
		},
		{
			name: "filters out-of-range indices",
			output: &specview.TaxonomyOutput{
				Domains: []specview.TaxonomyDomain{
					{
						Name: "Auth",
						Features: []specview.TaxonomyFeature{
							{Name: "Login", FileIndices: []int{0, -1, 99}},
						},
					},
				},
			},
			fileCount: 3,
			want:      map[int]bool{0: true},
		},
		{
			name: "multiple domains and features",
			output: &specview.TaxonomyOutput{
				Domains: []specview.TaxonomyDomain{
					{
						Name: "Auth",
						Features: []specview.TaxonomyFeature{
							{Name: "Login", FileIndices: []int{0}},
							{Name: "Logout", FileIndices: []int{1}},
						},
					},
					{
						Name: "Payment",
						Features: []specview.TaxonomyFeature{
							{Name: "Stripe", FileIndices: []int{2, 3}},
						},
					},
				},
			},
			fileCount: 4,
			want:      map[int]bool{0: true, 1: true, 2: true, 3: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectCoveredIndices(tt.output, tt.fileCount)
			if len(got) != len(tt.want) {
				t.Errorf("expected %d indices, got %d", len(tt.want), len(got))
			}
			for idx := range tt.want {
				if !got[idx] {
					t.Errorf("expected index %d to be covered", idx)
				}
			}
		})
	}
}
