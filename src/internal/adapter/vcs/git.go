package vcs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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

// GetHeadCommit returns the HEAD commit info (SHA and visibility) using git ls-remote.
// It determines visibility by trying unauthenticated access first:
// - Success without token = public repository (IsPrivate=false)
// - Failure without token, success with token = private repository (IsPrivate=true)
func (v *GitVCS) GetHeadCommit(ctx context.Context, url string, token *string) (analysis.CommitInfo, error) {
	if url == "" {
		return analysis.CommitInfo{}, fmt.Errorf("get head commit: URL is required")
	}

	sha, err := v.lsRemote(ctx, url, nil)
	if err == nil {
		return analysis.CommitInfo{SHA: sha, IsPrivate: false}, nil
	}

	if token == nil {
		return analysis.CommitInfo{}, fmt.Errorf("git ls-remote %q: %w", url, err)
	}

	sha, err = v.lsRemote(ctx, url, token)
	if err != nil {
		return analysis.CommitInfo{}, fmt.Errorf("git ls-remote %q: %w", url, err)
	}

	return analysis.CommitInfo{SHA: sha, IsPrivate: true}, nil
}

func (v *GitVCS) lsRemote(ctx context.Context, url string, token *string) (string, error) {
	targetURL := url
	if token != nil {
		targetURL = strings.Replace(url, "https://", fmt.Sprintf("https://x-access-token:%s@", *token), 1)
	}

	cmd := exec.CommandContext(ctx, "git", "ls-remote", targetURL, "HEAD")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"HOME=/nonexistent",
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w", stderr.String(), err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", fmt.Errorf("empty response")
	}

	parts := strings.Fields(output)
	if len(parts) < 1 {
		return "", fmt.Errorf("unexpected output format: %s", output)
	}

	return parts[0], nil
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

func (a *gitSourceAdapter) CommittedAt() time.Time {
	return a.gitSrc.CommittedAt()
}

func (a *gitSourceAdapter) Close(_ context.Context) error {
	return a.gitSrc.Close()
}

// VerifyCommitExists checks if a commit SHA exists in the remote repository
// by running "git fetch --depth 1 origin <sha>" on the cloned repository.
//
// Returns:
//   - (true, nil): commit exists
//   - (false, nil): commit does not exist (git reports "not our ref")
//   - (false, error): verification failed (network error, context cancelled, etc.)
//
// Note: "not our ref" detection may vary across git versions/locales.
func (a *gitSourceAdapter) VerifyCommitExists(ctx context.Context, sha string) (bool, error) {
	if sha == "" {
		return false, fmt.Errorf("verify commit exists: SHA is required")
	}

	cmd := exec.CommandContext(ctx, "git", "fetch", "--depth", "1", "origin", sha)
	cmd.Dir = a.gitSrc.Root()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return false, fmt.Errorf("verify commit exists %s: %w", sha, ctx.Err())
		}
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "not our ref") {
			return false, nil
		}
		return false, fmt.Errorf("git fetch origin %s: %s: %w", sha, stderrStr, err)
	}

	return true, nil
}

// CoreSource returns the underlying source.Source for use by the parser adapter.
// This allows the parser to access the core source interface without exposing
// implementation details in the domain layer.
func (a *gitSourceAdapter) CoreSource() source.Source {
	return a.gitSrc
}
