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
			{Index: 10, Name: "TestLogin_ValidCredentials"},
			{Index: 20, Name: "TestLogin_InvalidPassword"},
		},
	}

	prompt, indexMapping := BuildPhase2UserPrompt(input, "English")

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

	// Check test format uses 0-based indices for AI
	if !strings.Contains(prompt, "0|TestLogin_ValidCredentials") {
		t.Error("prompt should contain first test with 0-based index")
	}
	if !strings.Contains(prompt, "1|TestLogin_InvalidPassword") {
		t.Error("prompt should contain second test with 0-based index")
	}

	// Check index mapping preserves original indices
	if len(indexMapping) != 2 {
		t.Fatalf("expected 2 index mappings, got %d", len(indexMapping))
	}
	if indexMapping[0] != 10 {
		t.Errorf("expected indexMapping[0] = 10, got %d", indexMapping[0])
	}
	if indexMapping[1] != 20 {
		t.Errorf("expected indexMapping[1] = 20, got %d", indexMapping[1])
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

	prompt, indexMapping := BuildPhase2UserPrompt(input, "Korean")

	// Check test indices are converted to 0-based for AI
	if !strings.Contains(prompt, "0|TestRegister_Success") {
		t.Error("prompt should use 0-based index for first test")
	}
	if !strings.Contains(prompt, "1|TestRegister_DuplicateEmail") {
		t.Error("prompt should use 0-based index for second test")
	}
	if !strings.Contains(prompt, "2|TestRegister_WeakPassword") {
		t.Error("prompt should use 0-based index for third test")
	}

	// Check index mapping preserves original indices
	if len(indexMapping) != 3 {
		t.Fatalf("expected 3 index mappings, got %d", len(indexMapping))
	}
	expectedMapping := []int{5, 6, 7}
	for i, expected := range expectedMapping {
		if indexMapping[i] != expected {
			t.Errorf("expected indexMapping[%d] = %d, got %d", i, expected, indexMapping[i])
		}
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
		prompt, _ := BuildPhase2UserPrompt(input, lang)
		expected := fmt.Sprintf("Target Language: %s", lang)
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt for %s should contain %q", lang, expected)
		}
	}
}

func TestPhase2SystemPrompt_ContainsRequiredSections(t *testing.T) {
	requiredSections := []string{
		"Constraints",
		"Process",
		"Specification Notation",
		"Confidence",
		"Output",
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
	// Prompt now uses Korean as primary example, others apply same principle
	if !strings.Contains(Phase2SystemPrompt, "Korean") {
		t.Error("system prompt should contain Korean as primary example")
	}
}
