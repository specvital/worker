package analysis

import (
	"context"
	"errors"
	"fmt"
)

var ErrCodebaseNotFound = errors.New("codebase not found")

type Codebase struct {
	ExternalRepoID string
	Host           string
	ID             UUID
	IsStale        bool
	LastCommitSHA  string
	Name           string
	Owner          string
}

type UpsertCodebaseParams struct {
	DefaultBranch  string
	ExternalRepoID string
	Host           string
	Name           string
	Owner          string
}

func (p UpsertCodebaseParams) Validate() error {
	if p.Host == "" {
		return fmt.Errorf("%w: host is required", ErrInvalidInput)
	}
	if p.Owner == "" {
		return fmt.Errorf("%w: owner is required", ErrInvalidInput)
	}
	if p.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if p.ExternalRepoID == "" {
		return fmt.Errorf("%w: external repo ID is required", ErrInvalidInput)
	}
	return nil
}

type CodebaseRepository interface {
	FindByExternalID(ctx context.Context, host, externalRepoID string) (*Codebase, error)
	FindByOwnerName(ctx context.Context, host, owner, name string) (*Codebase, error)
	FindWithLastCommit(ctx context.Context, host, owner, name string) (*Codebase, error)
	MarkStale(ctx context.Context, id UUID) error
	UnmarkStale(ctx context.Context, id UUID, owner, name string) (*Codebase, error)
	UpdateOwnerName(ctx context.Context, id UUID, owner, name string) (*Codebase, error)
	Upsert(ctx context.Context, params UpsertCodebaseParams) (*Codebase, error)
}
