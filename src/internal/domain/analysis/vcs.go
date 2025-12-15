package analysis

import "context"

type VCS interface {
	Clone(ctx context.Context, url string) (Source, error)
}

type Source interface {
	Branch() string
	CommitSHA() string
	Close(ctx context.Context) error
}
