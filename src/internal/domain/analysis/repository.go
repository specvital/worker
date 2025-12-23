package analysis

import (
	"context"
	"fmt"
)

type Repository interface {
	CreateAnalysisRecord(ctx context.Context, params CreateAnalysisRecordParams) (UUID, error)
	RecordFailure(ctx context.Context, analysisID UUID, errMessage string) error
	SaveAnalysisInventory(ctx context.Context, params SaveAnalysisInventoryParams) error
}

type CreateAnalysisRecordParams struct {
	AnalysisID     *UUID
	Branch         string
	CodebaseID     *UUID
	CommitSHA      string
	ExternalRepoID string
	Owner          string
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
	return nil
}

type SaveAnalysisInventoryParams struct {
	AnalysisID UUID
	Inventory  *Inventory
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
