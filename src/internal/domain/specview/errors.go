package specview

import "errors"

var (
	ErrAIUnavailable    = errors.New("AI service unavailable")
	ErrAnalysisNotFound = errors.New("analysis not found")
	ErrDocumentNotFound = errors.New("document not found")
	ErrInvalidInput     = errors.New("invalid input")
	ErrRateLimited      = errors.New("rate limit exceeded")
)
