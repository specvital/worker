package batch

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultPollerConfig(t *testing.T) {
	config := DefaultPollerConfig()

	if config.InitialInterval != 30*time.Second {
		t.Errorf("InitialInterval = %v, expected 30s", config.InitialInterval)
	}
	if config.MaxInterval != 5*time.Minute {
		t.Errorf("MaxInterval = %v, expected 5m", config.MaxInterval)
	}
	if config.MaxWaitTime != 0 {
		t.Errorf("MaxWaitTime = %v, expected 0", config.MaxWaitTime)
	}
	if config.Multiplier != 1.5 {
		t.Errorf("Multiplier = %v, expected 1.5", config.Multiplier)
	}
}

func TestPoller_handleTerminalState(t *testing.T) {
	poller := &Poller{}

	tests := []struct {
		name        string
		result      *BatchResult
		wantErr     error
		wantSuccess bool
	}{
		{
			name: "succeeded returns result without error",
			result: &BatchResult{
				JobName: "test-job",
				State:   JobStateSucceeded,
			},
			wantErr:     nil,
			wantSuccess: true,
		},
		{
			name: "failed returns ErrJobFailed",
			result: &BatchResult{
				JobName: "test-job",
				State:   JobStateFailed,
			},
			wantErr:     ErrJobFailed,
			wantSuccess: false,
		},
		{
			name: "failed with error includes error message",
			result: &BatchResult{
				JobName: "test-job",
				State:   JobStateFailed,
				Error:   errors.New("internal error"),
			},
			wantErr:     ErrJobFailed,
			wantSuccess: false,
		},
		{
			name: "cancelled returns ErrJobCancelled",
			result: &BatchResult{
				JobName: "test-job",
				State:   JobStateCancelled,
			},
			wantErr:     ErrJobCancelled,
			wantSuccess: false,
		},
		{
			name: "expired returns ErrJobExpired",
			result: &BatchResult{
				JobName: "test-job",
				State:   JobStateExpired,
			},
			wantErr:     ErrJobExpired,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := poller.handleTerminalState(tt.result)

			if tt.wantSuccess {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("expected result, got nil")
				}
			} else {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestShouldRetryJob(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "job expired is not retryable",
			err:  ErrJobExpired,
			want: false,
		},
		{
			name: "job cancelled is not retryable",
			err:  ErrJobCancelled,
			want: false,
		},
		{
			name: "job failed is retryable",
			err:  ErrJobFailed,
			want: true,
		},
		{
			name: "context canceled is not retryable",
			err:  context.Canceled,
			want: false,
		},
		{
			name: "context deadline exceeded is not retryable",
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "network error is retryable",
			err:  errors.New("network timeout"),
			want: true,
		},
		{
			name: "wrapped job expired is not retryable",
			err:  errors.Join(errors.New("outer"), ErrJobExpired),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldRetryJob(tt.err)
			if got != tt.want {
				t.Errorf("ShouldRetryJob() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPoller_PollOnce_EmptyJobName(t *testing.T) {
	poller := &Poller{}

	_, err := poller.PollOnce(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty job name")
	}
}

func TestPoller_PollUntilComplete_EmptyJobName(t *testing.T) {
	poller := &Poller{}

	_, err := poller.PollUntilComplete(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty job name")
	}
}
