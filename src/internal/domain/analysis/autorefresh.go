package analysis

import (
	"context"
	"time"
)

type AutoRefreshRepository interface {
	GetCodebasesForAutoRefresh(ctx context.Context) ([]CodebaseRefreshInfo, error)
}

type CodebaseRefreshInfo struct {
	ConsecutiveFailures int
	Host                string
	ID                  UUID
	LastCommitSHA       string
	LastCompletedAt     *time.Time
	LastParserVersion   string
	LastViewedAt        time.Time
	Name                string
	Owner               string
}

type TaskQueue interface {
	EnqueueAnalysis(ctx context.Context, owner, repo, commitSHA string) error
}
