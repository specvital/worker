package queue

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/specvital/worker/internal/adapter/queue/analyze"
)

// Client is insert-only (no worker).
type Client struct {
	client *river.Client[pgx.Tx]
}

func NewClient(ctx context.Context, pool *pgxpool.Pool) (*Client, error) {
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return nil, err
	}

	return &Client{
		client: client,
	}, nil
}

func (c *Client) Close() error {
	// river.Client doesn't need explicit close for insert-only mode
	return nil
}

func (c *Client) EnqueueAnalysis(ctx context.Context, owner, repo, commitSHA string) error {
	_, err := c.client.Insert(ctx, analyze.AnalyzeArgs{
		Owner:     owner,
		Repo:      repo,
		CommitSHA: commitSHA,
	}, &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	})
	return err
}

func (c *Client) EnqueueAnalysisWithUser(ctx context.Context, owner, repo, commitSHA string, userID *string) error {
	_, err := c.client.Insert(ctx, analyze.AnalyzeArgs{
		Owner:     owner,
		Repo:      repo,
		CommitSHA: commitSHA,
		UserID:    userID,
	}, &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	})
	return err
}
