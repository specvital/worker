package mapping

import (
	"testing"

	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/specvital/core/pkg/domain"
)

func TestConvertCoreToDomainInventory_Nil(t *testing.T) {
	result := ConvertCoreToDomainInventory(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestConvertCoreToDomainInventory_Empty(t *testing.T) {
	coreInv := &domain.Inventory{
		Files:    []domain.TestFile{},
		RootPath: "/test",
	}

	result := ConvertCoreToDomainInventory(coreInv)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Files) != 0 {
		t.Errorf("expected empty files, got %d files", len(result.Files))
	}
}

func TestConvertCoreTestStatus(t *testing.T) {
	tests := []struct {
		name       string
		coreStatus domain.TestStatus
		expected   analysis.TestStatus
	}{
		{
			name:       "active",
			coreStatus: domain.TestStatusActive,
			expected:   analysis.TestStatusActive,
		},
		{
			name:       "skipped",
			coreStatus: domain.TestStatusSkipped,
			expected:   analysis.TestStatusSkipped,
		},
		{
			name:       "todo",
			coreStatus: domain.TestStatusTodo,
			expected:   analysis.TestStatusTodo,
		},
		{
			name:       "xfail maps to todo",
			coreStatus: domain.TestStatusXfail,
			expected:   analysis.TestStatusTodo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertCoreTestStatus(tt.coreStatus)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConvertCoreTestFile(t *testing.T) {
	coreFile := domain.TestFile{
		Path:      "test.ts",
		Framework: "jest",
		Language:  domain.LanguageTypeScript,
		Suites:    []domain.TestSuite{},
		Tests: []domain.Test{
			{
				Name: "test 1",
				Location: domain.Location{
					StartLine: 10,
					EndLine:   15,
				},
				Status: domain.TestStatusActive,
			},
		},
	}

	result := convertCoreTestFile(coreFile)

	if result.Path != "test.ts" {
		t.Errorf("expected path 'test.ts', got %s", result.Path)
	}
	if result.Framework != "jest" {
		t.Errorf("expected framework 'jest', got %s", result.Framework)
	}
	if len(result.Tests) != 1 {
		t.Errorf("expected 1 test, got %d", len(result.Tests))
	}
	if result.Tests[0].Name != "test 1" {
		t.Errorf("expected test name 'test 1', got %s", result.Tests[0].Name)
	}
}

func TestConvertCoreTestSuite(t *testing.T) {
	coreSuite := domain.TestSuite{
		Name: "suite 1",
		Location: domain.Location{
			StartLine: 5,
			EndLine:   20,
		},
		Suites: []domain.TestSuite{
			{
				Name: "nested suite",
				Location: domain.Location{
					StartLine: 10,
					EndLine:   15,
				},
			},
		},
		Tests: []domain.Test{
			{
				Name: "test in suite",
				Location: domain.Location{
					StartLine: 12,
					EndLine:   13,
				},
				Status: domain.TestStatusSkipped,
			},
		},
	}

	result := convertCoreTestSuite(coreSuite)

	if result.Name != "suite 1" {
		t.Errorf("expected name 'suite 1', got %s", result.Name)
	}
	if result.Location.StartLine != 5 {
		t.Errorf("expected start line 5, got %d", result.Location.StartLine)
	}
	if len(result.Suites) != 1 {
		t.Errorf("expected 1 nested suite, got %d", len(result.Suites))
	}
	if len(result.Tests) != 1 {
		t.Errorf("expected 1 test, got %d", len(result.Tests))
	}
	if result.Tests[0].Status != analysis.TestStatusSkipped {
		t.Errorf("expected status skipped, got %v", result.Tests[0].Status)
	}
}
