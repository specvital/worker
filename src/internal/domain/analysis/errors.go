package analysis

import "errors"

var (
	ErrAlreadyCompleted = errors.New("analysis already completed")
	ErrInvalidInput     = errors.New("invalid input")
	ErrRepoNotFound     = errors.New("repository not found")
)
