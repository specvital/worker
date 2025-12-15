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
func (v *GitVCS) Clone(ctx context.Context, url string) (analysis.Source, error) {
	if url == "" {
		return nil, fmt.Errorf("clone repository: URL is required")
	}

	gitSrc, err := source.NewGitSource(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("clone repository %q: %w", url, err)
	}

	return &gitSourceAdapter{gitSrc: gitSrc}, nil
}

// gitSourceAdapter adapts source.GitSource to implement analysis.Source.
// The main difference is that our domain Source.Close accepts a context,
// while the underlying GitSource.Close does not.
type gitSourceAdapter struct {
	gitSrc *source.GitSource
}

func (a *gitSourceAdapter) Branch() string {
	return a.gitSrc.Branch()
}

func (a *gitSourceAdapter) CommitSHA() string {
	return a.gitSrc.CommitSHA()
}

// Close implements analysis.Source.Close.
// The context parameter is accepted for interface compatibility but is not used
// by the underlying GitSource.Close implementation. Close operations are typically
// fast (temp directory cleanup) and don't benefit from cancellation.
func (a *gitSourceAdapter) Close(_ context.Context) error {
	return a.gitSrc.Close()
}

// unwrapGitSource returns the underlying source.GitSource.
// This is used by the parser adapter to access the source.Source interface
// required by parser.Scan.
func (a *gitSourceAdapter) unwrapGitSource() *source.GitSource {
	return a.gitSrc
}
