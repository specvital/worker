package vcs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/specvital/collector/internal/domain/analysis"
)

const (
	gitHubHost    = "github.com"
	gitHubAPIBase = "https://api.github.com"
)

type GitHubAPIClient struct {
	apiBase    string
	httpClient *http.Client
}

var _ analysis.VCSAPIClient = (*GitHubAPIClient)(nil)

func NewGitHubAPIClient(httpClient *http.Client) *GitHubAPIClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &GitHubAPIClient{
		apiBase:    gitHubAPIBase,
		httpClient: httpClient,
	}
}

func (c *GitHubAPIClient) GetRepoID(ctx context.Context, host, owner, repo string, token *string) (string, error) {
	if host != gitHubHost {
		return "", fmt.Errorf("%w: unsupported host %q (only %q is supported)", analysis.ErrInvalidInput, host, gitHubHost)
	}
	if owner == "" {
		return "", fmt.Errorf("%w: owner is required", analysis.ErrInvalidInput)
	}
	if repo == "" {
		return "", fmt.Errorf("%w: repo is required", analysis.ErrInvalidInput)
	}

	url := fmt.Sprintf("%s/repos/%s/%s", c.apiBase, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != nil && *token != "" {
		req.Header.Set("Authorization", "Bearer "+*token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get repository %s/%s: %w", owner, repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("%w: %s/%s", analysis.ErrRepoNotFound, owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get repository %s/%s: unexpected status %d", owner, repo, resp.StatusCode)
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return strconv.FormatInt(result.ID, 10), nil
}
