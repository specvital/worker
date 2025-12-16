package vcs

import (
	"context"
	"strings"
	"testing"
)

func TestNewGitVCS(t *testing.T) {
	vcs := NewGitVCS()
	if vcs == nil {
		t.Fatal("NewGitVCS returned nil")
	}
}

func TestGitVCS_Clone_EmptyURL(t *testing.T) {
	vcs := NewGitVCS()
	_, err := vcs.Clone(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
	if !strings.Contains(err.Error(), "URL is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGitSourceAdapter_Interface(t *testing.T) {
	// This test verifies that gitSourceAdapter implements the expected methods
	// without needing an actual GitSource (compile-time check)
	var adapter *gitSourceAdapter

	// These calls will panic if called on nil, but we're just checking compilation
	// analysis.Source methods
	_ = func() string { return adapter.Branch() }
	_ = func() string { return adapter.CommitSHA() }
	_ = func() error { return adapter.Close(context.Background()) }
	// coreSourceProvider method
	_ = func() interface{} { return adapter.CoreSource() }
}
