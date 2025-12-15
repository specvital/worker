package analysis

import "context"

type Repository interface {
	CreateAnalysisRecord(ctx context.Context, params CreateAnalysisRecordParams) (UUID, error)
	RecordFailure(ctx context.Context, analysisID UUID, errMessage string) error
	SaveAnalysisInventory(ctx context.Context, params SaveAnalysisInventoryParams) error
}

type CreateAnalysisRecordParams struct {
	Branch    string
	CommitSHA string
	Owner     string
	Repo      string
}

type SaveAnalysisInventoryParams struct {
	AnalysisID UUID
	Inventory  *Inventory
}
