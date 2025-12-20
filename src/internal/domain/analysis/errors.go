package analysis

import "errors"

var (
	ErrInvalidInput     = errors.New("invalid input")
	ErrAlreadyCompleted = errors.New("analysis already completed")
)
