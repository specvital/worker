package analysis

import (
	"fmt"
	"strings"
)

type AnalyzeRequest struct {
	Owner string
	Repo  string
}

func (r AnalyzeRequest) Validate() error {
	if r.Owner == "" {
		return fmt.Errorf("%w: owner is required", ErrInvalidInput)
	}
	if r.Repo == "" {
		return fmt.Errorf("%w: repo is required", ErrInvalidInput)
	}
	if len(r.Owner) > 39 || len(r.Repo) > 100 {
		return fmt.Errorf("%w: owner/repo exceeds length limit", ErrInvalidInput)
	}
	if !isValidGitHubName(r.Owner) || !isValidGitHubName(r.Repo) {
		return fmt.Errorf("%w: invalid characters in owner/repo", ErrInvalidInput)
	}
	return nil
}

func isValidGitHubName(s string) bool {
	if s == "" {
		return false
	}
	if s == "." || s == ".." || strings.Contains(s, "..") {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}
