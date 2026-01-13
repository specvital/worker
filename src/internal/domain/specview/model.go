package specview

import (
	"fmt"
	"time"
)

// Language represents supported languages for spec-view generation.
type Language string

const (
	LanguageEN Language = "en"
	LanguageJA Language = "ja"
	LanguageKO Language = "ko"
)

// SpecViewRequest represents a request to generate a spec-view document.
type SpecViewRequest struct {
	AnalysisID string
	Language   Language
	ModelID    string // optional: AI model override
}

func (r SpecViewRequest) Validate() error {
	if r.AnalysisID == "" {
		return fmt.Errorf("%w: analysis ID is required", ErrInvalidInput)
	}
	if r.Language == "" {
		return fmt.Errorf("%w: language is required", ErrInvalidInput)
	}
	if !r.Language.IsValid() {
		return fmt.Errorf("%w: unsupported language: %s", ErrInvalidInput, r.Language)
	}
	return nil
}

// IsValid checks if the language is one of the supported values.
func (l Language) IsValid() bool {
	switch l {
	case LanguageEN, LanguageJA, LanguageKO:
		return true
	default:
		return false
	}
}

// SpecViewResult represents the result of spec-view generation.
type SpecViewResult struct {
	CacheHit    bool
	ContentHash []byte
	DocumentID  string
}

// Phase1Input represents input for domain classification (Phase 1).
type Phase1Input struct {
	Files []FileInfo
}

// FileInfo represents a test file with its tests.
type FileInfo struct {
	Path  string
	Tests []TestInfo
}

// TestInfo represents a single test within a file.
type TestInfo struct {
	Index     int    // unique identifier for cross-referencing in Phase1Output.FeatureGroup.TestIndices
	Name      string
	SuitePath string // nested suite path (e.g., "SuiteA > SuiteB")
}

// Phase1Output represents the result of domain classification.
type Phase1Output struct {
	Domains []DomainGroup
}

// DomainGroup represents a classified domain with its features.
type DomainGroup struct {
	Confidence  float64
	Description string
	Features    []FeatureGroup
	Name        string
}

// FeatureGroup represents a feature within a domain.
type FeatureGroup struct {
	Confidence  float64
	Description string
	Name        string
	TestIndices []int // references to Phase1Input.Files[*].Tests[*].Index
}

// Phase2Input represents input for test name conversion (Phase 2).
type Phase2Input struct {
	DomainContext string // domain context for better conversion
	FeatureName   string
	Tests         []TestForConversion
}

// TestForConversion represents a test to be converted.
type TestForConversion struct {
	Index int
	Name  string
}

// Phase2Output represents the result of test name conversion.
type Phase2Output struct {
	Behaviors []BehaviorSpec
}

// BehaviorSpec represents a converted test behavior.
type BehaviorSpec struct {
	Confidence  float64
	Description string
	TestIndex   int
}

// SpecDocument represents the final spec-view document (4-table hierarchy root).
type SpecDocument struct {
	AnalysisID  string
	ContentHash []byte
	CreatedAt   time.Time
	Domains     []Domain
	ID          string
	Language    Language
	ModelID     string
}

// Domain represents a domain within a spec document.
type Domain struct {
	Confidence  float64
	Description string
	Features    []Feature
	ID          string
	Name        string
}

// Feature represents a feature within a domain.
type Feature struct {
	Behaviors   []Behavior
	Confidence  float64
	Description string
	ID          string
	Name        string
}

// Behavior represents a behavior (converted test) within a feature.
type Behavior struct {
	Confidence  float64
	Description string
	ID          string
	TestCaseID  string // FK to test_cases table
}
