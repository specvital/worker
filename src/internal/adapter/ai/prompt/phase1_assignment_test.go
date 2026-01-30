package prompt

import (
	"strings"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestBuildAssignmentUserPrompt_IncludesTaxonomy(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{
				Name:        "Authentication",
				Description: "User authentication",
				Features: []specview.TaxonomyFeature{
					{Name: "Login", FileIndices: []int{0}},
					{Name: "Session Management", FileIndices: []int{1}},
				},
			},
			{
				Name:        "Payment",
				Description: "Payment processing",
				Features: []specview.TaxonomyFeature{
					{Name: "Checkout", FileIndices: []int{2}},
				},
			},
		},
	}

	batch := specview.AssignmentBatch{
		BatchIndex: 0,
		Tests: []specview.TestForAssignment{
			{Index: 0, FilePath: "auth/login_test.go", Name: "should validate credentials"},
		},
	}

	prompt := BuildAssignmentUserPrompt(batch, taxonomy, "English")

	if !strings.Contains(prompt, "<taxonomy>") {
		t.Error("prompt should contain taxonomy section")
	}
	if !strings.Contains(prompt, "- Authentication") {
		t.Error("prompt should contain domain name")
	}
	if !strings.Contains(prompt, "  - Login") {
		t.Error("prompt should contain feature name with indentation")
	}
	if !strings.Contains(prompt, "  - Session Management") {
		t.Error("prompt should contain all features")
	}
	if !strings.Contains(prompt, "- Payment") {
		t.Error("prompt should contain second domain")
	}
	if !strings.Contains(prompt, "  - Checkout") {
		t.Error("prompt should contain second domain's features")
	}
	if !strings.Contains(prompt, "</taxonomy>") {
		t.Error("prompt should close taxonomy section")
	}
	// Check valid-pairs section is included
	if !strings.Contains(prompt, "<valid-pairs>") {
		t.Error("prompt should contain valid-pairs section")
	}
	if !strings.Contains(prompt, `"Authentication" / "Login"`) {
		t.Error("prompt should list valid pairs with exact names")
	}
	if !strings.Contains(prompt, `"Uncategorized" / "General"`) {
		t.Error("prompt should always include Uncategorized/General pair")
	}
}

func TestBuildAssignmentUserPrompt_CompactFormat(t *testing.T) {
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
		BatchIndex: 0,
		Tests: []specview.TestForAssignment{
			{Index: 0, FilePath: "auth_test.go", Name: "should login"},
			{Index: 1, FilePath: "auth_test.go", Name: "should logout", SuitePath: "UserSession"},
		},
	}

	prompt := BuildAssignmentUserPrompt(batch, taxonomy, "Korean")

	if !strings.Contains(prompt, "[0] auth_test.go: should login") {
		t.Error("prompt should format tests with index, path, and name")
	}
	if !strings.Contains(prompt, "[1] auth_test.go: should logout (suite: UserSession)") {
		t.Error("prompt should include suite path when present")
	}
	if !strings.Contains(prompt, "Total: 2 tests") {
		t.Error("prompt should include test count")
	}
	if !strings.Contains(prompt, "indices 0-1") {
		t.Error("prompt should include index range")
	}
	if !strings.Contains(prompt, "EVERY TEST") {
		t.Error("prompt should clarify that every test must be assigned")
	}
}

func TestBuildAssignmentUserPrompt_EmptyTaxonomy(t *testing.T) {
	batch := specview.AssignmentBatch{
		BatchIndex: 0,
		Tests: []specview.TestForAssignment{
			{Index: 0, FilePath: "test.go", Name: "test"},
		},
	}

	prompt := BuildAssignmentUserPrompt(batch, nil, "English")

	if !strings.Contains(prompt, "(empty)") {
		t.Error("prompt should handle nil taxonomy gracefully")
	}

	prompt = BuildAssignmentUserPrompt(batch, &specview.TaxonomyOutput{}, "English")

	if !strings.Contains(prompt, "(empty)") {
		t.Error("prompt should handle empty taxonomy gracefully")
	}
}

func TestBuildAssignmentUserPrompt_EmptyTests(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Domain", Features: []specview.TaxonomyFeature{{Name: "Feature"}}},
		},
	}

	batch := specview.AssignmentBatch{
		BatchIndex: 0,
		Tests:      []specview.TestForAssignment{},
	}

	prompt := BuildAssignmentUserPrompt(batch, taxonomy, "English")

	if !strings.Contains(prompt, "Total: 0 tests") {
		t.Error("prompt should handle empty tests")
	}
	if !strings.Contains(prompt, "No tests to assign") {
		t.Error("prompt should indicate no tests to assign")
	}
}

func TestBuildAssignmentUserPrompt_SingleTest(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Domain", Features: []specview.TaxonomyFeature{{Name: "Feature"}}},
		},
	}

	batch := specview.AssignmentBatch{
		BatchIndex: 0,
		Tests: []specview.TestForAssignment{
			{Index: 5, FilePath: "test.go", Name: "single test"},
		},
	}

	prompt := BuildAssignmentUserPrompt(batch, taxonomy, "English")

	if !strings.Contains(prompt, "[5]") {
		t.Error("prompt should display the actual test index in entry")
	}
	if !strings.Contains(prompt, "Total: 1 test") {
		t.Error("prompt should show singular test count")
	}
	if !strings.Contains(prompt, "this test") {
		t.Error("prompt should use singular form for single test")
	}
	if strings.Contains(prompt, "indices") {
		t.Error("prompt should NOT use range format for single test")
	}
}

func TestBuildAssignmentUserPrompt_UnorderedIndices(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Domain", Features: []specview.TaxonomyFeature{{Name: "Feature"}}},
		},
	}

	batch := specview.AssignmentBatch{
		BatchIndex: 1,
		Tests: []specview.TestForAssignment{
			{Index: 150, FilePath: "b.go", Name: "test b"},
			{Index: 100, FilePath: "a.go", Name: "test a"},
			{Index: 199, FilePath: "c.go", Name: "test c"},
		},
	}

	prompt := BuildAssignmentUserPrompt(batch, taxonomy, "English")

	if !strings.Contains(prompt, "indices 100-199") {
		t.Error("prompt should find correct min-max even when tests are unordered")
	}
}

func TestBuildAssignmentUserPrompt_NonContiguousIndices(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "Domain", Features: []specview.TaxonomyFeature{{Name: "Feature"}}},
		},
	}

	batch := specview.AssignmentBatch{
		BatchIndex: 1,
		Tests: []specview.TestForAssignment{
			{Index: 100, FilePath: "a.go", Name: "test a"},
			{Index: 150, FilePath: "b.go", Name: "test b"},
			{Index: 199, FilePath: "c.go", Name: "test c"},
		},
	}

	prompt := BuildAssignmentUserPrompt(batch, taxonomy, "English")

	if !strings.Contains(prompt, "indices 100-199") {
		t.Error("prompt should show correct min-max index range")
	}
	if !strings.Contains(prompt, "[100]") {
		t.Error("prompt should include first test index")
	}
	if !strings.Contains(prompt, "[199]") {
		t.Error("prompt should include last test index")
	}
}

func TestBuildAssignmentUserPrompt_TokenEfficiency(t *testing.T) {
	domains := make([]specview.TaxonomyDomain, 20)
	for i := 0; i < 20; i++ {
		features := make([]specview.TaxonomyFeature, 5)
		for j := 0; j < 5; j++ {
			features[j] = specview.TaxonomyFeature{Name: "Feature Name Here"}
		}
		domains[i] = specview.TaxonomyDomain{
			Name:        "Domain Name Here For Testing",
			Description: "Some description for domain",
			Features:    features,
		}
	}

	taxonomy := &specview.TaxonomyOutput{Domains: domains}

	tests := make([]specview.TestForAssignment, 100)
	for i := 0; i < 100; i++ {
		tests[i] = specview.TestForAssignment{
			Index:     i,
			FilePath:  "src/domain/feature/component_test.go",
			Name:      "should perform some specific action when given input",
			SuitePath: "ParentSuite > ChildSuite",
		}
	}

	batch := specview.AssignmentBatch{
		BatchIndex: 0,
		Tests:      tests,
	}

	prompt := BuildAssignmentUserPrompt(batch, taxonomy, "English")

	tokens := estimateTokenCount(prompt)
	maxAllowed := 6400
	if tokens > maxAllowed {
		t.Errorf("prompt for 100 tests + 20 domains should be under %d tokens (80%% safety margin of 8K), got approximately %d", maxAllowed, tokens)
	}
}

func TestBuildAssignmentUserPrompt_LanguageHeader(t *testing.T) {
	taxonomy := &specview.TaxonomyOutput{
		Domains: []specview.TaxonomyDomain{
			{Name: "인증", Features: []specview.TaxonomyFeature{{Name: "로그인"}}},
		},
	}

	batch := specview.AssignmentBatch{
		BatchIndex: 0,
		Tests: []specview.TestForAssignment{
			{Index: 0, FilePath: "auth_test.go", Name: "should login"},
		},
	}

	prompt := BuildAssignmentUserPrompt(batch, taxonomy, "Korean")

	if !strings.Contains(prompt, "Target Language: Korean") {
		t.Error("prompt should contain target language")
	}
	if !strings.Contains(prompt, "- 인증") {
		t.Error("prompt should preserve Korean domain names from taxonomy")
	}
}

func TestPhase1AssignmentSystemPrompt_ContainsRequiredSections(t *testing.T) {
	requiredSections := []string{
		"Constraints",
		"EXACT domain and feature names",
		"Classification Priority",
		"Output Format",
		"compact field names",
		`"a"`,
		`"d"`,
		`"f"`,
		`"t"`,
		"Uncategorized",
		"Example",
	}

	for _, section := range requiredSections {
		if !strings.Contains(Phase1AssignmentSystemPrompt, section) {
			t.Errorf("assignment system prompt should contain %q", section)
		}
	}
}
