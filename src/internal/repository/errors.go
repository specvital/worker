package repository

import "errors"

// Sentinel errors for repository operations.
var (
	ErrInvalidParams = errors.New("invalid repository parameters")
)
