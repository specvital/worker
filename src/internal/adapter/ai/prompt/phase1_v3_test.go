package prompt

import (
	"strings"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestBuildV3BatchUserPrompt_EmptyTests(t *testing.T) {
	prompt := BuildV3BatchUserPrompt(nil, nil, "English")

	if !strings.Contains(prompt, "Total: 0 tests") {
		t.Error("prompt should handle empty tests")
	}
	if !strings.Contains(prompt, "No tests to classify") {
		t.Error("prompt should indicate no tests")
	}
}

func TestBuildV3BatchUserPrompt_SingleTest(t *testing.T) {
	tests := []specview.TestForAssignment{
		{Index: 0, FilePath: "auth/login_test.go", Name: "should validate credentials"},
	}

	prompt := BuildV3BatchUserPrompt(tests, nil, "Korean")

	if !strings.Contains(prompt, "Target Language: Korean") {
		t.Error("prompt should contain target language")
	}
	if !strings.Contains(prompt, "[0] auth/login_test.go: should validate credentials") {
		t.Error("prompt should contain test entry")
	}
	if !strings.Contains(prompt, "Total: 1 test") {
		t.Error("prompt should indicate single test")
	}
	if !strings.Contains(prompt, "exactly 1 classification") {
		t.Error("prompt should require exactly 1 classification")
	}
}

func TestBuildV3BatchUserPrompt_MultipleTests(t *testing.T) {
	tests := []specview.TestForAssignment{
		{Index: 0, FilePath: "auth/login_test.go", Name: "should validate credentials"},
		{Index: 1, FilePath: "auth/login_test.go", Name: "should reject invalid password"},
		{Index: 2, FilePath: "payment/cart_test.go", Name: "should calculate total"},
	}

	prompt := BuildV3BatchUserPrompt(tests, nil, "English")

	if !strings.Contains(prompt, "[0] auth/login_test.go: should validate credentials") {
		t.Error("prompt should contain first test")
	}
	if !strings.Contains(prompt, "[1] auth/login_test.go: should reject invalid password") {
		t.Error("prompt should contain second test")
	}
	if !strings.Contains(prompt, "[2] payment/cart_test.go: should calculate total") {
		t.Error("prompt should contain third test")
	}
	if !strings.Contains(prompt, "Total: 3 tests") {
		t.Error("prompt should indicate test count")
	}
	if !strings.Contains(prompt, "exactly 3 classifications") {
		t.Error("prompt should require exact classification count")
	}
	if !strings.Contains(prompt, "same order") {
		t.Error("prompt should emphasize order preservation")
	}
}

func TestBuildV3BatchUserPrompt_WithSuitePath(t *testing.T) {
	tests := []specview.TestForAssignment{
		{
			Index:     0,
			FilePath:  "auth.spec.ts",
			Name:      "validates credentials",
			SuitePath: "LoginFlow > Validation",
		},
	}

	prompt := BuildV3BatchUserPrompt(tests, nil, "English")

	if !strings.Contains(prompt, "(suite: LoginFlow > Validation)") {
		t.Error("prompt should include suite path")
	}
}

func TestBuildV3BatchUserPrompt_WithoutExistingDomains(t *testing.T) {
	tests := []specview.TestForAssignment{
		{Index: 0, FilePath: "test.go", Name: "test"},
	}

	prompt := BuildV3BatchUserPrompt(tests, nil, "English")

	if !strings.Contains(prompt, "<existing-domains>") {
		t.Error("prompt should have existing-domains section")
	}
	if !strings.Contains(prompt, "(none - create new domains as needed)") {
		t.Error("prompt should indicate no existing domains")
	}
}

func TestBuildV3BatchUserPrompt_WithExistingDomains(t *testing.T) {
	tests := []specview.TestForAssignment{
		{Index: 0, FilePath: "test.go", Name: "test"},
	}
	existingDomains := []DomainSummary{
		{
			Name:        "Authentication",
			Description: "User authentication and authorization",
			Features:    []string{"Login", "Session Management", "Password Reset"},
		},
		{
			Name:        "Payment",
			Description: "Payment processing",
			Features:    []string{"Checkout", "Refund"},
		},
	}

	prompt := BuildV3BatchUserPrompt(tests, existingDomains, "English")

	if !strings.Contains(prompt, "Prefer assigning to these existing domains") {
		t.Error("prompt should instruct to prefer existing domains")
	}
	if !strings.Contains(prompt, "- Authentication: User authentication and authorization") {
		t.Error("prompt should contain domain with description")
	}
	if !strings.Contains(prompt, "  - Login") {
		t.Error("prompt should contain features")
	}
	if !strings.Contains(prompt, "  - Session Management") {
		t.Error("prompt should contain all features")
	}
	if !strings.Contains(prompt, "- Payment: Payment processing") {
		t.Error("prompt should contain second domain")
	}
}

func TestBuildV3BatchUserPrompt_ExistingDomainWithoutDescription(t *testing.T) {
	tests := []specview.TestForAssignment{
		{Index: 0, FilePath: "test.go", Name: "test"},
	}
	existingDomains := []DomainSummary{
		{
			Name:     "Authentication",
			Features: []string{"Login"},
		},
	}

	prompt := BuildV3BatchUserPrompt(tests, existingDomains, "English")

	if strings.Contains(prompt, "Authentication:") {
		t.Error("prompt should not have colon when no description")
	}
	if !strings.Contains(prompt, "- Authentication\n") {
		t.Error("prompt should contain domain name without description")
	}
}

func TestPhase1V3SystemPrompt_ContainsRequiredSections(t *testing.T) {
	requiredSections := []string{
		"Order-Based Mapping",
		"STRICT REQUIREMENT",
		"exactly N",
		"Classification Principles",
		"existing domains",
		"Uncategorized",
		"Output Format",
		`"d"`,
		`"f"`,
		"Language",
	}

	for _, section := range requiredSections {
		if !strings.Contains(Phase1V3SystemPrompt, section) {
			t.Errorf("V3 system prompt should contain %q", section)
		}
	}
}

func TestPhase1V3SystemPrompt_OrderEmphasis(t *testing.T) {
	orderKeywords := []string{
		"SAME ORDER",
		"position 0",
		"position 1",
	}

	for _, keyword := range orderKeywords {
		if !strings.Contains(Phase1V3SystemPrompt, keyword) {
			t.Errorf("V3 system prompt should emphasize order with %q", keyword)
		}
	}
}
