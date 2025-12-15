package parser

import (
	"context"
	"strings"
	"testing"

	"github.com/specvital/collector/internal/domain/analysis"
)

func TestNewCoreParser(t *testing.T) {
	parser := NewCoreParser()
	if parser == nil {
		t.Fatal("NewCoreParser returned nil")
	}
}

func TestCoreParser_Scan_InvalidSourceType(t *testing.T) {
	parser := NewCoreParser()

	// mockSource doesn't implement gitSourceUnwrapper
	mockSrc := &mockInvalidSource{}

	_, err := parser.Scan(context.Background(), mockSrc)
	if err == nil {
		t.Fatal("expected error for source without unwrapper")
	}
	if !strings.Contains(err.Error(), "does not support unwrapping") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// mockInvalidSource implements analysis.Source but not gitSourceUnwrapper.
type mockInvalidSource struct{}

var _ analysis.Source = (*mockInvalidSource)(nil)

func (m *mockInvalidSource) Branch() string              { return "" }
func (m *mockInvalidSource) CommitSHA() string           { return "" }
func (m *mockInvalidSource) Close(_ context.Context) error { return nil }

// Conversion tests moved to adapter/mapping/core_domain_test.go
