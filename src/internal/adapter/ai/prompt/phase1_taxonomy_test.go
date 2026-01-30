package prompt

import (
	"fmt"
	"strings"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestBuildTaxonomyUserPrompt_BasicFormat(t *testing.T) {
	input := specview.TaxonomyInput{
		AnalysisID: "test-analysis-id",
		Language:   "English",
		Files: []specview.TaxonomyFileInfo{
			{
				Index:     0,
				Path:      "src/auth/login_test.go",
				TestCount: 5,
			},
			{
				Index:     1,
				Path:      "src/user/profile_test.go",
				TestCount: 3,
			},
		},
	}

	prompt := BuildTaxonomyUserPrompt(input)

	if !strings.Contains(prompt, "Target Language: English") {
		t.Error("prompt should contain target language")
	}
	if !strings.Contains(prompt, "[0] src/auth/login_test.go (5 tests)") {
		t.Error("prompt should contain file path with test count")
	}
	if !strings.Contains(prompt, "[1] src/user/profile_test.go (3 tests)") {
		t.Error("prompt should contain second file")
	}
	if !strings.Contains(prompt, "Total: 2 files") {
		t.Error("prompt should contain total file count")
	}
	if !strings.Contains(prompt, "indices 0-1") {
		t.Error("prompt should contain file index range")
	}
	if !strings.Contains(prompt, "EVERY file") {
		t.Error("prompt should clarify that every file must be assigned")
	}
}

func TestBuildTaxonomyUserPrompt_WithDomainHints(t *testing.T) {
	input := specview.TaxonomyInput{
		AnalysisID: "test-analysis-id",
		Language:   "Korean",
		Files: []specview.TaxonomyFileInfo{
			{
				Index:     0,
				Path:      "auth.spec.ts",
				TestCount: 10,
				DomainHints: &specview.DomainHints{
					Imports: []string{"AuthService", "UserRepository"},
					Calls:   []string{"authService.login", "userRepo.findById"},
				},
			},
		},
	}

	prompt := BuildTaxonomyUserPrompt(input)

	if !strings.Contains(prompt, "imports: AuthService, UserRepository") {
		t.Error("prompt should contain imports")
	}
	if !strings.Contains(prompt, "calls: authService.login, userRepo.findById") {
		t.Error("prompt should contain calls")
	}
}

func TestBuildTaxonomyUserPrompt_NoTestNames(t *testing.T) {
	input := specview.TaxonomyInput{
		AnalysisID: "test-analysis-id",
		Language:   "English",
		Files: []specview.TaxonomyFileInfo{
			{
				Index:     0,
				Path:      "auth_test.go",
				TestCount: 100,
			},
		},
	}

	prompt := BuildTaxonomyUserPrompt(input)

	if strings.Contains(prompt, "tests:") {
		t.Error("taxonomy prompt should NOT contain test names section")
	}
	if strings.Contains(prompt, "TestLogin") {
		t.Error("taxonomy prompt should NOT contain individual test names")
	}
}

func TestBuildTaxonomyUserPrompt_EmptyFiles(t *testing.T) {
	input := specview.TaxonomyInput{
		AnalysisID: "test-analysis-id",
		Language:   "English",
		Files:      []specview.TaxonomyFileInfo{},
	}

	prompt := BuildTaxonomyUserPrompt(input)

	if !strings.Contains(prompt, "Total: 0 files") {
		t.Error("prompt should handle empty files gracefully")
	}
	if !strings.Contains(prompt, "No files to process") {
		t.Error("prompt should indicate no files to process")
	}
}

func TestBuildTaxonomyUserPrompt_SingleFile(t *testing.T) {
	input := specview.TaxonomyInput{
		AnalysisID: "test-analysis-id",
		Language:   "English",
		Files: []specview.TaxonomyFileInfo{
			{Index: 0, Path: "test.go", TestCount: 5},
		},
	}

	prompt := BuildTaxonomyUserPrompt(input)

	if !strings.Contains(prompt, "index 0") {
		t.Error("prompt should reference single file correctly")
	}
	if !strings.Contains(prompt, "this file") {
		t.Error("prompt should use singular form for single file")
	}
	if strings.Contains(prompt, "indices 0-0") {
		t.Error("prompt should NOT use range format for single file")
	}
}

func TestBuildTaxonomyUserPrompt_LargeDomainHints(t *testing.T) {
	largeImports := make([]string, 50)
	for i := 0; i < 50; i++ {
		largeImports[i] = fmt.Sprintf("github.com/org/package%d", i)
	}

	largeCalls := make([]string, 50)
	for i := 0; i < 50; i++ {
		largeCalls[i] = fmt.Sprintf("service.Method%d", i)
	}

	input := specview.TaxonomyInput{
		AnalysisID: "test-analysis-id",
		Language:   "English",
		Files: []specview.TaxonomyFileInfo{
			{
				Index:     0,
				Path:      "test.go",
				TestCount: 5,
				DomainHints: &specview.DomainHints{
					Imports: largeImports,
					Calls:   largeCalls,
				},
			},
		},
	}

	prompt := BuildTaxonomyUserPrompt(input)

	if strings.Contains(prompt, "package49") {
		t.Error("prompt should limit imports to MaxDomainHintsPerType")
	}
	if strings.Contains(prompt, "Method49") {
		t.Error("prompt should limit calls to MaxDomainHintsPerType")
	}
	if !strings.Contains(prompt, "package9") {
		t.Error("prompt should include imports up to limit")
	}
	if !strings.Contains(prompt, "Method9") {
		t.Error("prompt should include calls up to limit")
	}
}

func TestBuildTaxonomyUserPrompt_TokenEfficiency(t *testing.T) {
	files := make([]specview.TaxonomyFileInfo, 500)
	for i := 0; i < 500; i++ {
		files[i] = specview.TaxonomyFileInfo{
			Index:     i,
			Path:      "src/domain/feature/component_test.go",
			TestCount: 20,
			DomainHints: &specview.DomainHints{
				Imports: []string{"ServiceA", "ServiceB", "ServiceC"},
				Calls:   []string{"svc.Method1", "svc.Method2"},
			},
		}
	}

	input := specview.TaxonomyInput{
		AnalysisID: "test-analysis-id",
		Language:   "English",
		Files:      files,
	}

	prompt := BuildTaxonomyUserPrompt(input)

	tokens := estimateTokenCount(prompt)
	maxAllowed := 12000
	if tokens > maxAllowed {
		t.Errorf("prompt for 500 files should be under %d tokens (80%% safety margin of 15K), got approximately %d", maxAllowed, tokens)
	}
}

func TestPhase1TaxonomySystemPrompt_ContainsRequiredSections(t *testing.T) {
	requiredSections := []string{
		"Constraints",
		"Classification Priority",
		"Output",
		"domains",
		"features",
		"file_indices",
		"Uncategorized",
		"Example",
	}

	for _, section := range requiredSections {
		if !strings.Contains(Phase1TaxonomySystemPrompt, section) {
			t.Errorf("taxonomy system prompt should contain %q", section)
		}
	}
}
