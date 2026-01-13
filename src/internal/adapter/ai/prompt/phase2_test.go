package prompt

import (
	"fmt"
	"strings"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestBuildPhase2UserPrompt_BasicFormat(t *testing.T) {
	input := specview.Phase2Input{
		DomainContext: "Authentication",
		FeatureName:   "Login",
		Tests: []specview.TestForConversion{
			{Index: 0, Name: "TestLogin_ValidCredentials"},
			{Index: 1, Name: "TestLogin_InvalidPassword"},
		},
	}

	prompt := BuildPhase2UserPrompt(input, "English")

	// Check context
	if !strings.Contains(prompt, "Domain: Authentication") {
		t.Error("prompt should contain domain context")
	}
	if !strings.Contains(prompt, "Feature: Login") {
		t.Error("prompt should contain feature name")
	}
	if !strings.Contains(prompt, "Target Language: English") {
		t.Error("prompt should contain target language")
	}

	// Check test format
	if !strings.Contains(prompt, "0|TestLogin_ValidCredentials") {
		t.Error("prompt should contain first test")
	}
	if !strings.Contains(prompt, "1|TestLogin_InvalidPassword") {
		t.Error("prompt should contain second test")
	}
}

func TestBuildPhase2UserPrompt_MultipleTests(t *testing.T) {
	input := specview.Phase2Input{
		DomainContext: "User Management",
		FeatureName:   "Registration",
		Tests: []specview.TestForConversion{
			{Index: 5, Name: "TestRegister_Success"},
			{Index: 6, Name: "TestRegister_DuplicateEmail"},
			{Index: 7, Name: "TestRegister_WeakPassword"},
		},
	}

	prompt := BuildPhase2UserPrompt(input, "Korean")

	// Check test indices are preserved
	if !strings.Contains(prompt, "5|TestRegister_Success") {
		t.Error("prompt should preserve test index 5")
	}
	if !strings.Contains(prompt, "6|TestRegister_DuplicateEmail") {
		t.Error("prompt should preserve test index 6")
	}
	if !strings.Contains(prompt, "7|TestRegister_WeakPassword") {
		t.Error("prompt should preserve test index 7")
	}
}

func TestBuildPhase2UserPrompt_LanguageVariants(t *testing.T) {
	input := specview.Phase2Input{
		DomainContext: "Domain",
		FeatureName:   "Feature",
		Tests: []specview.TestForConversion{
			{Index: 0, Name: "Test1"},
		},
	}

	languages := []specview.Language{
		"English",
		"Korean",
		"Japanese",
	}

	for _, lang := range languages {
		prompt := BuildPhase2UserPrompt(input, lang)
		expected := fmt.Sprintf("Target Language: %s", lang)
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt for %s should contain %q", lang, expected)
		}
	}
}

func TestPhase2SystemPrompt_ContainsRequiredSections(t *testing.T) {
	requiredSections := []string{
		"Critical Constraints",
		"Conversion Process",
		"Language-Specific Style",
		"Confidence Scoring",
		"Output Format",
		"conversions",
		"index",
		"description",
		"confidence",
	}

	for _, section := range requiredSections {
		if !strings.Contains(Phase2SystemPrompt, section) {
			t.Errorf("system prompt should contain %q", section)
		}
	}
}

func TestPhase2SystemPrompt_ContainsLanguageStyles(t *testing.T) {
	languageStyles := []string{
		"Korean",
		"English",
		"Japanese",
	}

	for _, style := range languageStyles {
		if !strings.Contains(Phase2SystemPrompt, style) {
			t.Errorf("system prompt should contain style guide for %s", style)
		}
	}
}
