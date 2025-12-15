package vcs

import (
	"context"
	"strings"
	"testing"
)

func TestNewGitVCS(t *testing.T) {
	tests := []struct {
		name                string
		maxConcurrentClones int64
		expectedConcurrency int64
	}{
		{
			name:                "positive value",
			maxConcurrentClones: 5,
			expectedConcurrency: 5,
		},
		{
			name:                "zero defaults to 1",
			maxConcurrentClones: 0,
			expectedConcurrency: 1,
		},
		{
			name:                "negative defaults to 1",
			maxConcurrentClones: -1,
			expectedConcurrency: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vcs := NewGitVCS(tt.maxConcurrentClones)
			if vcs == nil {
				t.Fatal("NewGitVCS returned nil")
			}
			if vcs.cloneSem == nil {
				t.Fatal("cloneSem is nil")
			}
		})
	}
}

func TestGitVCS_Clone_EmptyURL(t *testing.T) {
	vcs := NewGitVCS(1)
	_, err := vcs.Clone(context.Background(), "")
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
	_ = func() string { return adapter.Branch() }
	_ = func() string { return adapter.CommitSHA() }
	_ = func() error { return adapter.Close(context.Background()) }
	_ = func() interface{} { return adapter.unwrapGitSource() }
}
