package analysis

import (
	"context"
	"errors"
)

var ErrCodebaseNotFound = errors.New("codebase not found")

type Codebase struct {
	ExternalRepoID string
	Host           string
	ID             UUID
	IsStale        bool
	Name           string
	Owner          string
}

type CodebaseRepository interface {
	FindByExternalID(ctx context.Context, host, externalRepoID string) (*Codebase, error)
	FindByOwnerName(ctx context.Context, host, owner, name string) (*Codebase, error)
	MarkStale(ctx context.Context, id UUID) error
	UnmarkStale(ctx context.Context, id UUID, owner, name string) (*Codebase, error)
	UpdateOwnerName(ctx context.Context, id UUID, owner, name string) (*Codebase, error)
}
