package prompt

import (
	"fmt"
	"strings"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestBuildPhase1UserPrompt_BasicFormat(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path:      "src/auth/login_test.go",
				Framework: "go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestLogin_ValidCredentials"},
					{Index: 1, Name: "TestLogin_InvalidPassword"},
				},
			},
		},
	}

	prompt := BuildPhase1UserPrompt(input, "English")

	if !strings.Contains(prompt, "Target Language: English") {
		t.Error("prompt should contain target language")
	}
	if !strings.Contains(prompt, "[0] src/auth/login_test.go (go)") {
		t.Error("prompt should contain file path with framework")
	}
	if !strings.Contains(prompt, "0|TestLogin_ValidCredentials") {
		t.Error("prompt should contain test name with index")
	}
	if !strings.Contains(prompt, "Total: 2 tests") {
		t.Error("prompt should contain total test count")
	}
}

func TestBuildPhase1UserPrompt_WithDomainHints(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path:      "auth.spec.ts",
				Framework: "jest",
				DomainHints: &specview.DomainHints{
					Imports: []string{"AuthService", "UserRepository"},
					Calls:   []string{"authService.login", "userRepo.findById"},
				},
				Tests: []specview.TestInfo{
					{Index: 0, Name: "should login"},
				},
			},
		},
	}

	prompt := BuildPhase1UserPrompt(input, "Korean")

	if !strings.Contains(prompt, "imports: AuthService, UserRepository") {
		t.Error("prompt should contain imports")
	}
	if !strings.Contains(prompt, "calls: authService.login, userRepo.findById") {
		t.Error("prompt should contain calls")
	}
}

func TestBuildPhase1UserPrompt_WithSuitePath(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "auth.spec.ts",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "should login", SuitePath: "Auth > Login"},
				},
			},
		},
	}

	prompt := BuildPhase1UserPrompt(input, "Korean")

	if !strings.Contains(prompt, "0|Auth > Login|should login") {
		t.Error("prompt should contain suite path when present")
	}
}

func TestBuildPhase1UserPrompt_MultipleFiles(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "auth_test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "TestAuth1"},
				},
			},
			{
				Path: "user_test.go",
				Tests: []specview.TestInfo{
					{Index: 1, Name: "TestUser1"},
					{Index: 2, Name: "TestUser2"},
				},
			},
		},
	}

	prompt := BuildPhase1UserPrompt(input, "English")

	if !strings.Contains(prompt, "[0] auth_test.go") {
		t.Error("prompt should contain first file")
	}
	if !strings.Contains(prompt, "[1] user_test.go") {
		t.Error("prompt should contain second file")
	}
	if !strings.Contains(prompt, "Total: 3 tests") {
		t.Error("prompt should have correct total count")
	}
}

func TestBuildPhase1UserPrompt_LanguageVariants(t *testing.T) {
	input := specview.Phase1Input{
		Files: []specview.FileInfo{
			{
				Path: "test.go",
				Tests: []specview.TestInfo{
					{Index: 0, Name: "Test1"},
				},
			},
		},
	}

	languages := []specview.Language{
		"English",
		"Korean",
		"Japanese",
	}

	for _, lang := range languages {
		prompt := BuildPhase1UserPrompt(input, lang)
		expected := fmt.Sprintf("Target Language: %s", lang)
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt for %s should contain %q", lang, expected)
		}
	}
}

func TestPhase1SystemPrompt_ContainsRequiredSections(t *testing.T) {
	requiredSections := []string{
		"Constraints",
		"Classification Priority",
		"Confidence",
		"Output",
		"domains",
		"features",
		"test_indices",
	}

	for _, section := range requiredSections {
		if !strings.Contains(Phase1SystemPrompt, section) {
			t.Errorf("system prompt should contain %q", section)
		}
	}
}
