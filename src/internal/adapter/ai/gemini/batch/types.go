package batch

import (
	"errors"
	"time"

	"google.golang.org/genai"

	"github.com/specvital/worker/internal/domain/specview"
)

// BatchJobState represents the current state of a batch job.
type BatchJobState string

const (
	JobStatePending   BatchJobState = "JOB_STATE_PENDING"
	JobStateRunning   BatchJobState = "JOB_STATE_RUNNING"
	JobStateSucceeded BatchJobState = "JOB_STATE_SUCCEEDED"
	JobStateFailed    BatchJobState = "JOB_STATE_FAILED"
	JobStateCancelled BatchJobState = "JOB_STATE_CANCELLED"
	JobStateExpired   BatchJobState = "JOB_STATE_EXPIRED"
)

// IsTerminal returns true if the job state is terminal (succeeded, failed, cancelled, or expired).
func (s BatchJobState) IsTerminal() bool {
	return s == JobStateSucceeded || s == JobStateFailed || s == JobStateCancelled || s == JobStateExpired
}

// BatchConfig holds configuration for batch processing.
type BatchConfig struct {
	APIKey         string
	BatchThreshold int
	Phase1Model    string
	PollInterval   time.Duration
	UseBatchAPI    bool
}

// Validate validates the batch configuration.
func (c *BatchConfig) Validate() error {
	if c.APIKey == "" {
		return errors.New("batch: API key is required")
	}
	if c.Phase1Model == "" {
		return errors.New("batch: Phase1Model is required")
	}
	return nil
}

// mapJobState converts genai.JobState to BatchJobState.
func mapJobState(state genai.JobState) BatchJobState {
	switch state {
	case genai.JobStatePending:
		return JobStatePending
	case genai.JobStateRunning:
		return JobStateRunning
	case genai.JobStateSucceeded:
		return JobStateSucceeded
	case genai.JobStateFailed:
		return JobStateFailed
	case genai.JobStateCancelled:
		return JobStateCancelled
	default:
		return BatchJobState(state)
	}
}

// BatchRequest represents a batch job request for Phase 1 classification.
type BatchRequest struct {
	AnalysisID string                  // Analysis ID for tracking
	ChunkCount int                     // Number of chunks (0 or 1 = single request)
	Model      string                  // Model name (e.g., "gemini-2.5-flash")
	Requests   []*genai.InlinedRequest // Inline requests for batch processing
}

// BatchResult represents the result of a completed batch job.
type BatchResult struct {
	JobName       string                   // Batch job resource name
	State         BatchJobState            // Final job state
	Responses     []*genai.InlinedResponse // Inline responses from batch job
	Error         error                    // Error if job failed
	TokenUsage    *specview.TokenUsage     // Token usage metadata
}

// ClassificationJobResult represents the result of a classification batch job
// after parsing into Phase1Output.
type ClassificationJobResult struct {
	Output     *specview.Phase1Output
	TokenUsage *specview.TokenUsage
}
