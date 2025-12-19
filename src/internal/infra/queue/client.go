package queue

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	adapterqueue "github.com/specvital/collector/internal/adapter/queue"
)

// 2 hours > 1h cron interval, prevents duplicate tasks from cron jitter.
const deduplicationWindow = 2 * time.Hour

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

func (c *Client) EnqueueAnalysis(ctx context.Context, owner, repo string) error {
	_, err := c.client.Insert(ctx, adapterqueue.AnalyzeArgs{
		Owner: owner,
		Repo:  repo,
	}, &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: deduplicationWindow,
		},
	})
	return err
}

func (c *Client) EnqueueAnalysisWithID(ctx context.Context, analysisID, owner, repo string, userID *string) error {
	_, err := c.client.Insert(ctx, adapterqueue.AnalyzeArgs{
		AnalysisID: &analysisID,
		Owner:      owner,
		Repo:       repo,
		UserID:     userID,
	}, &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: deduplicationWindow,
		},
	})
	return err
}
