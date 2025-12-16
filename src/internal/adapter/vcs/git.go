package vcs

import (
	"context"
	"fmt"

	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/specvital/core/pkg/source"
)

// GitVCS implements analysis.VCS using specvital/core's GitSource.
// It is a thin, stateless adapter that delegates to the underlying source package.
// Concurrency control (semaphore) is managed by the use case layer, not here.
type GitVCS struct{}

// NewGitVCS creates a new GitVCS.
func NewGitVCS() *GitVCS {
	return &GitVCS{}
}

// Clone implements analysis.VCS by cloning a Git repository.
func (v *GitVCS) Clone(ctx context.Context, url string, token *string) (analysis.Source, error) {
	if url == "" {
		return nil, fmt.Errorf("clone repository: URL is required")
	}

	var opts *source.GitOptions
	if token != nil {
		opts = &source.GitOptions{
			Credentials: &source.GitCredentials{
				Username: "x-access-token",
				Password: *token,
			},
		}
	}

	gitSrc, err := source.NewGitSource(ctx, url, opts)
	if err != nil {
		return nil, fmt.Errorf("clone repository %q: %w", url, err)
	}

	return &gitSourceAdapter{gitSrc: gitSrc}, nil
}

// gitSourceAdapter adapts source.GitSource to implement analysis.Source.
// It also provides access to the underlying source.Source for parser integration.
type gitSourceAdapter struct {
	gitSrc *source.GitSource
}

func (a *gitSourceAdapter) Branch() string {
	return a.gitSrc.Branch()
}

func (a *gitSourceAdapter) CommitSHA() string {
	return a.gitSrc.CommitSHA()
}

func (a *gitSourceAdapter) Close(_ context.Context) error {
	return a.gitSrc.Close()
}

// CoreSource returns the underlying source.Source for use by the parser adapter.
// This allows the parser to access the core source interface without exposing
// implementation details in the domain layer.
func (a *gitSourceAdapter) CoreSource() source.Source {
	return a.gitSrc
}
