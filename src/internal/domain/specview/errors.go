package specview

import "errors"

var (
	ErrAIUnavailable    = errors.New("AI service unavailable")
	ErrAnalysisNotFound = errors.New("analysis not found")
	ErrInvalidInput     = errors.New("invalid input")
	ErrOutputTruncated  = errors.New("AI output truncated due to token limit")
	ErrRateLimited      = errors.New("rate limit exceeded")
)
