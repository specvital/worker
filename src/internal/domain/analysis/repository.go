package analysis

import (
	"context"
	"fmt"
	"time"
)

type Repository interface {
	CreateAnalysisRecord(ctx context.Context, params CreateAnalysisRecordParams) (UUID, error)
	RecordFailure(ctx context.Context, analysisID UUID, errMessage string) error
	SaveAnalysisInventory(ctx context.Context, params SaveAnalysisInventoryParams) error
}

// StreamingRepository extends Repository with batch-based storage for streaming pipeline.
type StreamingRepository interface {
	Repository
	FinalizeAnalysis(ctx context.Context, params FinalizeAnalysisParams) error
	SaveAnalysisBatch(ctx context.Context, params SaveAnalysisBatchParams) (*BatchStats, error)
}

type CreateAnalysisRecordParams struct {
	AnalysisID     *UUID
	Branch         string
	CodebaseID     *UUID
	CommitSHA      string
	ExternalRepoID string
	Owner          string
	ParserVersion  string
	Repo           string
}

func (p CreateAnalysisRecordParams) Validate() error {
	if p.Owner == "" {
		return fmt.Errorf("%w: owner is required", ErrInvalidInput)
	}
	if p.Repo == "" {
		return fmt.Errorf("%w: repo is required", ErrInvalidInput)
	}
	if p.Branch == "" {
		return fmt.Errorf("%w: branch is required", ErrInvalidInput)
	}
	if p.CommitSHA == "" {
		return fmt.Errorf("%w: commit SHA is required", ErrInvalidInput)
	}
	// ExternalRepoID: allows legacy placeholder for now, add required validation after GitHub API integration
	if p.AnalysisID != nil && *p.AnalysisID == NilUUID {
		return fmt.Errorf("%w: analysis ID cannot be nil UUID", ErrInvalidInput)
	}
	if p.ParserVersion == "" {
		return fmt.Errorf("%w: parser version is required", ErrInvalidInput)
	}
	return nil
}

type SaveAnalysisInventoryParams struct {
	AnalysisID  UUID
	CommittedAt time.Time
	Inventory   *Inventory
	UserID      *string
}

func (p SaveAnalysisInventoryParams) Validate() error {
	if p.AnalysisID == NilUUID {
		return fmt.Errorf("%w: analysis ID is required", ErrInvalidInput)
	}
	if p.Inventory == nil {
		return fmt.Errorf("%w: inventory is required", ErrInvalidInput)
	}
	return nil
}

// SaveAnalysisBatchParams contains parameters for saving a batch of test files.
type SaveAnalysisBatchParams struct {
	AnalysisID UUID
	Files      []TestFile
}

func (p SaveAnalysisBatchParams) Validate() error {
	if p.AnalysisID == NilUUID {
		return fmt.Errorf("%w: analysis ID is required", ErrInvalidInput)
	}
	if len(p.Files) == 0 {
		return fmt.Errorf("%w: files cannot be empty", ErrInvalidInput)
	}
	return nil
}

// FinalizeAnalysisParams contains parameters for finalizing a streaming analysis.
type FinalizeAnalysisParams struct {
	AnalysisID  UUID
	CommittedAt time.Time
	TotalSuites int
	TotalTests  int
	UserID      *string
}

func (p FinalizeAnalysisParams) Validate() error {
	if p.AnalysisID == NilUUID {
		return fmt.Errorf("%w: analysis ID is required", ErrInvalidInput)
	}
	if p.TotalSuites < 0 || p.TotalTests < 0 {
		return fmt.Errorf("%w: totals cannot be negative", ErrInvalidInput)
	}
	return nil
}

// BatchStats represents statistics from a batch save operation.
type BatchStats struct {
	FilesProcessed  int
	SuitesProcessed int
	TestsProcessed  int
}
