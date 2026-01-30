package specview

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSpecViewRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     SpecViewRequest
		wantErr error
	}{
		{
			name: "valid request with English",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "English",
				UserID:     "user-123",
			},
			wantErr: nil,
		},
		{
			name: "valid request with Korean",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "Korean",
				ModelID:    "gemini-2.5-flash",
				UserID:     "user-123",
			},
			wantErr: nil,
		},
		{
			name: "valid request with any language",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "Chinese",
				UserID:     "user-123",
			},
			wantErr: nil,
		},
		{
			name: "empty analysis ID",
			req: SpecViewRequest{
				AnalysisID: "",
				Language:   "English",
				UserID:     "user-123",
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "empty user ID",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "English",
				UserID:     "",
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "empty language",
			req: SpecViewRequest{
				AnalysisID: "analysis-123",
				Language:   "",
				UserID:     "user-123",
			},
			wantErr: ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr == nil && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLanguage_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		language Language
		want     bool
	}{
		{name: "English valid", language: "English", want: true},
		{name: "Korean valid", language: "Korean", want: true},
		{name: "Japanese valid", language: "Japanese", want: true},
		{name: "Chinese valid", language: "Chinese", want: true},
		{name: "Spanish valid", language: "Spanish", want: true},
		{name: "any string valid", language: "AnyLanguage", want: true},
		{name: "empty invalid", language: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.language.IsValid()
			if got != tt.want {
				t.Errorf("Language(%q).IsValid() = %v, want %v", tt.language, got, tt.want)
			}
		})
	}
}

func TestTaxonomyInput_Construction(t *testing.T) {
	input := TaxonomyInput{
		AnalysisID: "analysis-123",
		Language:   "Korean",
		Files: []TaxonomyFileInfo{
			{
				Index:     0,
				Path:      "src/auth/login_test.go",
				TestCount: 5,
				DomainHints: &DomainHints{
					Imports: []string{"auth", "jwt"},
					Calls:   []string{"Login", "ValidateToken"},
				},
			},
			{
				Index:     1,
				Path:      "src/user/profile_test.go",
				TestCount: 3,
			},
		},
	}

	if input.AnalysisID != "analysis-123" {
		t.Errorf("AnalysisID = %q, want %q", input.AnalysisID, "analysis-123")
	}
	if input.Language != "Korean" {
		t.Errorf("Language = %q, want %q", input.Language, "Korean")
	}
	if len(input.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(input.Files))
	}
	if input.Files[0].TestCount != 5 {
		t.Errorf("Files[0].TestCount = %d, want 5", input.Files[0].TestCount)
	}
	if input.Files[1].DomainHints != nil {
		t.Error("Files[1].DomainHints should be nil")
	}
}

func TestTaxonomyOutput_JSONSerialization(t *testing.T) {
	output := TaxonomyOutput{
		Domains: []TaxonomyDomain{
			{
				Name:        "Authentication",
				Description: "User authentication and authorization",
				Features: []TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0, 1}},
					{Name: "JWT Validation", FileIndices: []int{2}},
				},
			},
			{
				Name:        "User Management",
				Description: "User profile and settings",
				Features: []TaxonomyFeature{
					{Name: "Profile", FileIndices: []int{3, 4, 5}},
				},
			},
		},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded TaxonomyOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(decoded.Domains) != 2 {
		t.Errorf("len(Domains) = %d, want 2", len(decoded.Domains))
	}
	if decoded.Domains[0].Name != "Authentication" {
		t.Errorf("Domains[0].Name = %q, want %q", decoded.Domains[0].Name, "Authentication")
	}
	if len(decoded.Domains[0].Features) != 2 {
		t.Errorf("len(Domains[0].Features) = %d, want 2", len(decoded.Domains[0].Features))
	}
	if decoded.Domains[0].Features[0].FileIndices[1] != 1 {
		t.Errorf("Domains[0].Features[0].FileIndices[1] = %d, want 1", decoded.Domains[0].Features[0].FileIndices[1])
	}
}

func TestAssignmentBatch_Construction(t *testing.T) {
	batch := AssignmentBatch{
		BatchIndex: 0,
		Tests: []TestForAssignment{
			{Index: 0, Name: "should login successfully", FilePath: "auth/login_test.go", SuitePath: "LoginSuite"},
			{Index: 1, Name: "should reject invalid password", FilePath: "auth/login_test.go", SuitePath: "LoginSuite"},
			{Index: 2, Name: "should create user", FilePath: "user/create_test.go", SuitePath: ""},
		},
	}

	if batch.BatchIndex != 0 {
		t.Errorf("BatchIndex = %d, want 0", batch.BatchIndex)
	}
	if len(batch.Tests) != 3 {
		t.Errorf("len(Tests) = %d, want 3", len(batch.Tests))
	}
	if batch.Tests[0].SuitePath != "LoginSuite" {
		t.Errorf("Tests[0].SuitePath = %q, want %q", batch.Tests[0].SuitePath, "LoginSuite")
	}
	if batch.Tests[2].SuitePath != "" {
		t.Errorf("Tests[2].SuitePath = %q, want empty", batch.Tests[2].SuitePath)
	}
}

func TestAssignmentOutput_JSONSerialization(t *testing.T) {
	output := AssignmentOutput{
		Assignments: []TestAssignment{
			{Domain: "Authentication", Feature: "Login", TestIndices: []int{0, 1, 2}},
			{Domain: "User Management", Feature: "Profile", TestIndices: []int{3, 4}},
			{Domain: "Uncategorized", Feature: "General", TestIndices: []int{5}},
		},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Compact format verification
	if !strings.Contains(string(data), `"a":[`) {
		t.Errorf("expected compact 'a' key, got: %s", string(data))
	}
	if !strings.Contains(string(data), `"d":`) {
		t.Errorf("expected compact 'd' key, got: %s", string(data))
	}
	if !strings.Contains(string(data), `"f":`) {
		t.Errorf("expected compact 'f' key, got: %s", string(data))
	}
	if !strings.Contains(string(data), `"t":`) {
		t.Errorf("expected compact 't' key, got: %s", string(data))
	}

	var decoded AssignmentOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(decoded.Assignments) != 3 {
		t.Errorf("len(Assignments) = %d, want 3", len(decoded.Assignments))
	}
	if decoded.Assignments[0].Domain != "Authentication" {
		t.Errorf("Assignments[0].Domain = %q, want %q", decoded.Assignments[0].Domain, "Authentication")
	}
	if decoded.Assignments[2].Feature != "General" {
		t.Errorf("Assignments[2].Feature = %q, want %q", decoded.Assignments[2].Feature, "General")
	}
}
