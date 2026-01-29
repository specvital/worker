package batch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ErrJobExpired indicates the batch job has expired before completion.
var ErrJobExpired = errors.New("batch: job expired")

// ErrJobCancelled indicates the batch job was cancelled.
var ErrJobCancelled = errors.New("batch: job cancelled")

// ErrJobFailed indicates the batch job failed with an error.
var ErrJobFailed = errors.New("batch: job failed")

// PollerConfig configures the polling behavior.
type PollerConfig struct {
	InitialInterval time.Duration // initial polling interval
	MaxInterval     time.Duration // maximum polling interval
	MaxWaitTime     time.Duration // maximum total wait time (0 = no limit)
	Multiplier      float64       // backoff multiplier
}

// DefaultPollerConfig returns sensible defaults for polling.
func DefaultPollerConfig() PollerConfig {
	return PollerConfig{
		InitialInterval: 30 * time.Second,
		MaxInterval:     5 * time.Minute,
		MaxWaitTime:     0, // no limit, rely on Gemini's expiration
		Multiplier:      1.5,
	}
}

// Poller handles polling batch job status with exponential backoff.
type Poller struct {
	config   PollerConfig
	provider *Provider
}

// NewPoller creates a new Poller with the given configuration.
func NewPoller(provider *Provider, config PollerConfig) *Poller {
	return &Poller{
		config:   config,
		provider: provider,
	}
}

// PollUntilComplete polls the job status until it reaches a terminal state.
// Returns the final result or an error if the job failed, expired, or was cancelled.
func (p *Poller) PollUntilComplete(ctx context.Context, jobName string) (*BatchResult, error) {
	if jobName == "" {
		return nil, errors.New("batch: job name is required")
	}

	interval := p.config.InitialInterval
	startTime := time.Now()

	for {
		result, err := p.provider.GetJobStatus(ctx, jobName)
		if err != nil {
			return nil, fmt.Errorf("failed to get job status: %w", err)
		}

		slog.InfoContext(ctx, "batch job status",
			"job_name", jobName,
			"state", result.State,
			"elapsed", time.Since(startTime).Round(time.Second),
		)

		if result.State.IsTerminal() {
			return p.handleTerminalState(result)
		}

		// Check max wait time if configured
		if p.config.MaxWaitTime > 0 && time.Since(startTime) >= p.config.MaxWaitTime {
			return nil, fmt.Errorf("batch: polling timeout after %v", p.config.MaxWaitTime)
		}

		// Wait before next poll with exponential backoff
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		// Increase interval with backoff, capped at max
		interval = min(
			time.Duration(float64(interval)*p.config.Multiplier),
			p.config.MaxInterval,
		)
	}
}

// PollOnce checks the job status once and returns the result.
// Useful for non-blocking status checks in worker polling scenarios.
func (p *Poller) PollOnce(ctx context.Context, jobName string) (*BatchResult, error) {
	if jobName == "" {
		return nil, errors.New("batch: job name is required")
	}

	result, err := p.provider.GetJobStatus(ctx, jobName)
	if err != nil {
		return nil, fmt.Errorf("failed to get job status: %w", err)
	}

	if result.State.IsTerminal() {
		return p.handleTerminalState(result)
	}

	return result, nil
}

// handleTerminalState processes a terminal state result.
func (p *Poller) handleTerminalState(result *BatchResult) (*BatchResult, error) {
	switch result.State {
	case JobStateSucceeded:
		return result, nil

	case JobStateFailed:
		if result.Error != nil {
			return result, fmt.Errorf("%w: %v", ErrJobFailed, result.Error)
		}
		return result, ErrJobFailed

	case JobStateCancelled:
		return result, ErrJobCancelled

	case JobStateExpired:
		return result, ErrJobExpired

	default:
		return result, fmt.Errorf("batch: unexpected terminal state: %s", result.State)
	}
}

// ShouldRetryJob determines if a failed job should be retried based on the error.
func ShouldRetryJob(err error) bool {
	// Job expiration and cancellation are not retryable
	if errors.Is(err, ErrJobExpired) || errors.Is(err, ErrJobCancelled) {
		return false
	}

	// Job failures may be retryable (transient errors)
	if errors.Is(err, ErrJobFailed) {
		return true
	}

	// Context errors are not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Other errors (network, API) are potentially retryable
	return true
}
