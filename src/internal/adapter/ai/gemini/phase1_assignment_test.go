package gemini

import (
	"context"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestCreateAssignmentBatches_SplitBy100(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "file1_test.go",
				Tests: func() []specview.TestInfo {
					tests := make([]specview.TestInfo, 150)
					for i := 0; i < 150; i++ {
						tests[i] = specview.TestInfo{Index: i, Name: "test" + string(rune('0'+i%10))}
					}
					return tests
				}(),
			},
			{
				Path: "file2_test.go",
				Tests: func() []specview.TestInfo {
					tests := make([]specview.TestInfo, 100)
					for i := 0; i < 100; i++ {
						tests[i] = specview.TestInfo{Index: 150 + i, Name: "test" + string(rune('0'+i%10))}
					}
					return tests
				}(),
			},
		},
	}

	batches := createAssignmentBatches(input, 100)

	if len(batches) != 3 {
		t.Errorf("expected 3 batches for 250 tests, got %d", len(batches))
	}

	if len(batches[0].Tests) != 100 {
		t.Errorf("expected batch 0 to have 100 tests, got %d", len(batches[0].Tests))
	}
	if len(batches[1].Tests) != 100 {
		t.Errorf("expected batch 1 to have 100 tests, got %d", len(batches[1].Tests))
	}
	if len(batches[2].Tests) != 50 {
		t.Errorf("expected batch 2 to have 50 tests, got %d", len(batches[2].Tests))
	}

	if batches[0].BatchIndex != 0 || batches[1].BatchIndex != 1 || batches[2].BatchIndex != 2 {
		t.Error("batch indices should be sequential starting from 0")
	}
}

func TestCreateAssignmentBatches_PreservesTestMetadata(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "auth/login_test.go",
				Tests: []specview.TestInfo{
					{Index: 5, Name: "should validate", SuitePath: "LoginSuite"},
					{Index: 6, Name: "should reject"},
				},
			},
		},
	}

	batches := createAssignmentBatches(input, 100)

	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}

	tests := batches[0].Tests
	if len(tests) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(tests))
	}

	if tests[0].Index != 5 {
		t.Errorf("expected test[0].Index = 5, got %d", tests[0].Index)
	}
	if tests[0].FilePath != "auth/login_test.go" {
		t.Errorf("expected test[0].FilePath 'auth/login_test.go', got %q", tests[0].FilePath)
	}
	if tests[0].Name != "should validate" {
		t.Errorf("expected test[0].Name 'should validate', got %q", tests[0].Name)
	}
	if tests[0].SuitePath != "LoginSuite" {
		t.Errorf("expected test[0].SuitePath 'LoginSuite', got %q", tests[0].SuitePath)
	}

	if tests[1].SuitePath != "" {
		t.Errorf("expected test[1].SuitePath empty, got %q", tests[1].SuitePath)
	}
}

func TestCreateAssignmentBatches_EmptyInput(t *testing.T) {
	input := specview.Phase1Input{Files: []specview.FileInfo{}}

	batches := createAssignmentBatches(input, 100)

	if batches != nil {
		t.Errorf("expected nil for empty input, got %d batches", len(batches))
	}
}

func TestCreateAssignmentBatches_DefaultBatchSize(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "test1"},
					{Index: 1, Name: "test2"},
				},
			},
		},
	}

	t.Run("zero batch size uses default", func(t *testing.T) {
		batches := createAssignmentBatches(input, 0)
		if len(batches) != 1 {
			t.Errorf("expected 1 batch with default size, got %d", len(batches))
		}
	})

	t.Run("negative batch size uses default", func(t *testing.T) {
		batches := createAssignmentBatches(input, -5)
		if len(batches) != 1 {
			t.Errorf("expected 1 batch with negative size fallback, got %d", len(batches))
		}
	})
}

func TestParseAssignmentResponse_ValidJSON(t *testing.T) {
	jsonStr := `{
		"a": [
			{"d": "Authentication", "f": "Login", "t": [0, 1, 2]},
			{"d": "Payment", "f": "Checkout", "t": [3, 4]}
		]
	}`

	output, err := parseAssignmentResponse(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Assignments) != 2 {
		t.Errorf("expected 2 assignments, got %d", len(output.Assignments))
	}

	a0 := output.Assignments[0]
	if a0.Domain != "Authentication" {
		t.Errorf("expected domain 'Authentication', got %q", a0.Domain)
	}
	if a0.Feature != "Login" {
		t.Errorf("expected feature 'Login', got %q", a0.Feature)
	}
	if len(a0.TestIndices) != 3 {
		t.Errorf("expected 3 test indices, got %d", len(a0.TestIndices))
	}
}

func TestParseAssignmentResponse_InvalidJSON(t *testing.T) {
	_, err := parseAssignmentResponse("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseAssignmentResponse_EmptyAssignments(t *testing.T) {
	_, err := parseAssignmentResponse(`{"a": []}`)
	if err == nil {
		t.Error("expected error for empty assignments")
	}
}

func TestParseAssignmentResponse_EmptyDomain(t *testing.T) {
	jsonStr := `{"a": [{"d": "", "f": "Login", "t": [0]}]}`
	_, err := parseAssignmentResponse(jsonStr)
	if err == nil {
		t.Error("expected error for empty domain")
	}
}

func TestParseAssignmentResponse_EmptyFeature(t *testing.T) {
	jsonStr := `{"a": [{"d": "Auth", "f": "", "t": [0]}]}`
	_, err := parseAssignmentResponse(jsonStr)
	if err == nil {
		t.Error("expected error for empty feature")
	}
}

func TestParseAssignmentResponse_EmptyTestIndices(t *testing.T) {
	jsonStr := `{"a": [{"d": "Auth", "f": "Login", "t": []}]}`
	_, err := parseAssignmentResponse(jsonStr)
	if err == nil {
		t.Error("expected error for empty test indices")
	}
}

func TestValidateAssignments_AllTestsCovered(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0}},
				},
			},
		},
	}

	batch := specview.AssignmentBatch{
		Tests: []specview.TestForAssignment{
			{Index: 0},
			{Index: 1},
			{Index: 2},
		},
	}

	output := &specview.AssignmentOutput{
		Assignments: []specview.TestAssignment{
			{Domain: "Auth", Feature: "Login", TestIndices: []int{0, 1, 2}},
		},
	}

	err := validateAssignments(context.Background(), output, batch, taxonomy)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAssignments_MissingTests(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Auth", Features: []specview.TaxonomyFeature{{Name: "Login"}}},
		},
	}

	batch := specview.AssignmentBatch{
		Tests: []specview.TestForAssignment{
			{Index: 0},
			{Index: 1},
			{Index: 2},
		},
	}

	output := &specview.AssignmentOutput{
		Assignments: []specview.TestAssignment{
			{Domain: "Auth", Feature: "Login", TestIndices: []int{0}},
		},
	}

	err := validateAssignments(context.Background(), output, batch, taxonomy)
	if err == nil {
		t.Error("expected error for missing tests")
	}
}

func TestValidateAssignments_UnexpectedTestIndex(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Auth", Features: []specview.TaxonomyFeature{{Name: "Login"}}},
		},
	}

	batch := specview.AssignmentBatch{
		Tests: []specview.TestForAssignment{
			{Index: 0},
		},
	}

	output := &specview.AssignmentOutput{
		Assignments: []specview.TestAssignment{
			{Domain: "Auth", Feature: "Login", TestIndices: []int{0, 99}},
		},
	}

	err := validateAssignments(context.Background(), output, batch, taxonomy)
	if err == nil {
		t.Error("expected error for unexpected test index")
	}
}

func TestValidateAssignments_InvalidPairReturnsError(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Auth", Features: []specview.TaxonomyFeature{{Name: "Login"}}},
		},
	}

	batch := specview.AssignmentBatch{
		Tests: []specview.TestForAssignment{
			{Index: 0},
			{Index: 1},
		},
	}

	output := &specview.AssignmentOutput{
		Assignments: []specview.TestAssignment{
			{Domain: "InvalidDomain", Feature: "InvalidFeature", TestIndices: []int{0}},
			{Domain: "Auth", Feature: "Login", TestIndices: []int{1}},
		},
	}

	err := validateAssignments(context.Background(), output, batch, taxonomy)
	if err == nil {
		t.Error("expected error for invalid domain/feature pair")
	}
}

func TestValidateAssignments_NilOutput(t *testing.T) {
	err := validateAssignments(context.Background(), nil, specview.AssignmentBatch{}, nil)
	if err == nil {
		t.Error("expected error for nil output")
	}
}

func TestBuildValidPairs(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name: "Auth",
				Features: []specview.TaxonomyFeature{
					{Name: "Login"},
					{Name: "Logout"},
				},
			},
			{
				Name: "Payment",
				Features: []specview.TaxonomyFeature{
					{Name: "Checkout"},
				},
			},
		},
	}

	pairs := buildValidPairs(taxonomy)

	// Keys are normalized (lowercase)
	expected := []string{
		"auth/login",
		"auth/logout",
		"payment/checkout",
		"uncategorized/general",
	}

	for _, pair := range expected {
		if !pairs[pair] {
			t.Errorf("expected pair %q to be valid", pair)
		}
	}

	if pairs["auth/invalid"] {
		t.Error("unexpected pair 'auth/invalid' marked as valid")
	}
}

func TestBuildValidPairs_NilTaxonomy(t *testing.T) {
	pairs := buildValidPairs(nil)

	// Key is normalized (lowercase)
	if !pairs["uncategorized/general"] {
		t.Error("should always include uncategorized/general")
	}
	if len(pairs) != 1 {
		t.Errorf("expected only 1 pair for nil taxonomy, got %d", len(pairs))
	}
}

func TestRecoverInvalidAssignments_FixesInvalidPairs(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Auth", Features: []specview.TaxonomyFeature{{Name: "Login"}}},
		},
	}

	batch := specview.AssignmentBatch{
		Tests: []specview.TestForAssignment{
			{Index: 0},
			{Index: 1},
			{Index: 2},
		},
	}

	output := &specview.AssignmentOutput{
		Assignments: []specview.TestAssignment{
			{Domain: "Auth", Feature: "Login", TestIndices: []int{0}},
			{Domain: "InvalidDomain", Feature: "InvalidFeature", TestIndices: []int{1}},
		},
	}

	recovered := recoverInvalidAssignments(output, batch, taxonomy)

	authLoginCount := 0
	uncategorizedCount := 0

	for _, a := range recovered.Assignments {
		if a.Domain == "Auth" && a.Feature == "Login" {
			authLoginCount = len(a.TestIndices)
		}
		if a.Domain == uncategorizedDomainName && a.Feature == uncategorizedFeatureName {
			uncategorizedCount = len(a.TestIndices)
		}
	}

	if authLoginCount != 1 {
		t.Errorf("expected 1 test in Auth/Login, got %d", authLoginCount)
	}
	if uncategorizedCount != 2 {
		t.Errorf("expected 2 tests in Uncategorized/General (1 invalid + 1 missing), got %d", uncategorizedCount)
	}
}

func TestRecoverInvalidAssignments_AddsMissingTests(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Auth", Features: []specview.TaxonomyFeature{{Name: "Login"}}},
		},
	}

	batch := specview.AssignmentBatch{
		Tests: []specview.TestForAssignment{
			{Index: 0},
			{Index: 1},
			{Index: 2},
		},
	}

	output := &specview.AssignmentOutput{
		Assignments: []specview.TestAssignment{
			{Domain: "Auth", Feature: "Login", TestIndices: []int{0}},
		},
	}

	recovered := recoverInvalidAssignments(output, batch, taxonomy)

	var uncategorizedIndices []int
	for _, a := range recovered.Assignments {
		if a.Domain == uncategorizedDomainName {
			uncategorizedIndices = a.TestIndices
		}
	}

	if len(uncategorizedIndices) != 2 {
		t.Errorf("expected 2 missing tests in Uncategorized, got %d", len(uncategorizedIndices))
	}
}

func TestRecoverInvalidAssignments_IgnoresDuplicates(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Auth", Features: []specview.TaxonomyFeature{{Name: "Login"}}},
		},
	}

	batch := specview.AssignmentBatch{
		Tests: []specview.TestForAssignment{
			{Index: 0},
			{Index: 1},
		},
	}

	output := &specview.AssignmentOutput{
		Assignments: []specview.TestAssignment{
			{Domain: "Auth", Feature: "Login", TestIndices: []int{0, 1}},
			{Domain: "Auth", Feature: "Login", TestIndices: []int{0}},
		},
	}

	recovered := recoverInvalidAssignments(output, batch, taxonomy)

	totalIndices := 0
	for _, a := range recovered.Assignments {
		totalIndices += len(a.TestIndices)
	}

	if totalIndices != 2 {
		t.Errorf("expected 2 total indices (no duplicates), got %d", totalIndices)
	}
}

func TestRecoverInvalidAssignments_IgnoresUnexpectedIndices(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Auth", Features: []specview.TaxonomyFeature{{Name: "Login"}}},
		},
	}

	batch := specview.AssignmentBatch{
		Tests: []specview.TestForAssignment{
			{Index: 0},
		},
	}

	output := &specview.AssignmentOutput{
		Assignments: []specview.TestAssignment{
			{Domain: "Auth", Feature: "Login", TestIndices: []int{0, 99}},
		},
	}

	recovered := recoverInvalidAssignments(output, batch, taxonomy)

	if len(recovered.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(recovered.Assignments))
	}
	if len(recovered.Assignments[0].TestIndices) != 1 {
		t.Errorf("expected 1 test index (excluding 99), got %d", len(recovered.Assignments[0].TestIndices))
	}
}
