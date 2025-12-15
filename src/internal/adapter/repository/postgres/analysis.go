package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/specvital/collector/internal/infra/db"
	"github.com/specvital/core/pkg/parser"
)

const defaultHost = "github.com"
const maxErrorMessageLength = 1000

func truncateErrorMessage(msg string) string {
	if len(msg) <= maxErrorMessageLength {
		return msg
	}
	return msg[:maxErrorMessageLength-15] + "... (truncated)"
}

// SaveAnalysisResultParams is used internally for the convenience method SaveAnalysisResult.
// It combines CreateAnalysisRecord and SaveAnalysisInventory into a single operation.
type SaveAnalysisResultParams struct {
	Branch    string
	CommitSHA string
	Owner     string
	Repo      string
	Result    *parser.ScanResult
}

func (p SaveAnalysisResultParams) Validate() error {
	domainParams := analysis.CreateAnalysisRecordParams{
		Owner:     p.Owner,
		Repo:      p.Repo,
		CommitSHA: p.CommitSHA,
		Branch:    p.Branch,
	}
	if err := domainParams.Validate(); err != nil {
		return err
	}
	if p.Result == nil {
		return fmt.Errorf("%w: result is required", analysis.ErrInvalidInput)
	}
	return nil
}

// AnalysisRepository implements the domain's analysis.Repository interface
// using PostgreSQL as the persistence layer.
type AnalysisRepository struct {
	pool *pgxpool.Pool
}

// NewAnalysisRepository creates a new PostgreSQL-backed analysis repository.
func NewAnalysisRepository(pool *pgxpool.Pool) *AnalysisRepository {
	return &AnalysisRepository{pool: pool}
}

// CreateAnalysisRecord implements analysis.Repository.
func (r *AnalysisRepository) CreateAnalysisRecord(ctx context.Context, params analysis.CreateAnalysisRecordParams) (analysis.UUID, error) {
	if err := params.Validate(); err != nil {
		return analysis.NilUUID, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return analysis.NilUUID, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			slog.ErrorContext(ctx, "failed to rollback transaction",
				"operation", "CreateAnalysisRecord",
				"error", rbErr,
				"owner", params.Owner,
				"repo", params.Repo,
			)
		}
	}()

	queries := db.New(tx)
	startedAt := time.Now()

	codebase, err := queries.UpsertCodebase(ctx, db.UpsertCodebaseParams{
		Host:          defaultHost,
		Owner:         params.Owner,
		Name:          params.Repo,
		DefaultBranch: pgtype.Text{String: params.Branch, Valid: params.Branch != ""},
	})
	if err != nil {
		return analysis.NilUUID, fmt.Errorf("upsert codebase: %w", err)
	}

	dbAnalysis, err := queries.CreateAnalysis(ctx, db.CreateAnalysisParams{
		CodebaseID: codebase.ID,
		CommitSha:  params.CommitSHA,
		BranchName: pgtype.Text{String: params.Branch, Valid: params.Branch != ""},
		Status:     db.AnalysisStatusRunning,
		StartedAt:  pgtype.Timestamptz{Time: startedAt, Valid: true},
	})
	if err != nil {
		return analysis.NilUUID, fmt.Errorf("create analysis: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return analysis.NilUUID, fmt.Errorf("commit transaction: %w", err)
	}

	return fromPgUUID(dbAnalysis.ID), nil
}

// RecordFailure implements analysis.Repository.
func (r *AnalysisRepository) RecordFailure(ctx context.Context, analysisID analysis.UUID, errMessage string) error {
	if analysisID == analysis.NilUUID {
		return fmt.Errorf("%w: analysis ID is required", analysis.ErrInvalidInput)
	}
	if errMessage == "" {
		return fmt.Errorf("%w: error message is required", analysis.ErrInvalidInput)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			slog.ErrorContext(ctx, "failed to rollback transaction",
				"operation", "RecordFailure",
				"error", rbErr,
				"analysis_id", analysisID,
			)
		}
	}()

	queries := db.New(tx)
	truncatedMsg := truncateErrorMessage(errMessage)
	pgID := toPgUUID(analysisID)

	if err := queries.UpdateAnalysisFailed(ctx, db.UpdateAnalysisFailedParams{
		ID:           pgID,
		ErrorMessage: pgtype.Text{String: truncatedMsg, Valid: true},
		CompletedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("update analysis failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// SaveAnalysisInventory implements analysis.Repository.
func (r *AnalysisRepository) SaveAnalysisInventory(ctx context.Context, params analysis.SaveAnalysisInventoryParams) error {
	if err := params.Validate(); err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			slog.ErrorContext(ctx, "failed to rollback transaction",
				"operation", "SaveAnalysisInventory",
				"error", rbErr,
				"analysis_id", params.AnalysisID,
			)
		}
	}()

	queries := db.New(tx)
	pgID := toPgUUID(params.AnalysisID)

	totalSuites, totalTests, err := r.saveInventory(ctx, queries, pgID, params.Inventory)
	if err != nil {
		return fmt.Errorf("save inventory: %w", err)
	}

	if err := queries.UpdateAnalysisCompleted(ctx, db.UpdateAnalysisCompletedParams{
		ID:          pgID,
		TotalSuites: int32(totalSuites),
		TotalTests:  int32(totalTests),
		CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("update analysis: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// SaveAnalysisResult is a convenience method that combines CreateAnalysisRecord and SaveAnalysisInventory.
// This method is kept for backward compatibility with existing code that uses parser.ScanResult.
// It is not part of the domain interface.
func (r *AnalysisRepository) SaveAnalysisResult(ctx context.Context, params SaveAnalysisResultParams) error {
	if err := params.Validate(); err != nil {
		return err
	}

	analysisID, err := r.CreateAnalysisRecord(ctx, analysis.CreateAnalysisRecordParams{
		Branch:    params.Branch,
		CommitSHA: params.CommitSHA,
		Owner:     params.Owner,
		Repo:      params.Repo,
	})
	if err != nil {
		return err
	}

	domainInventory := convertCoreToDomainInventory(params.Result.Inventory)

	if err := r.SaveAnalysisInventory(ctx, analysis.SaveAnalysisInventoryParams{
		AnalysisID: analysisID,
		Inventory:  domainInventory,
	}); err != nil {
		return err
	}

	return nil
}

func (r *AnalysisRepository) saveInventory(ctx context.Context, queries *db.Queries, analysisID pgtype.UUID, inventory *analysis.Inventory) (int, int, error) {
	if inventory == nil {
		return 0, 0, nil
	}

	var totalSuites, totalTests int
	for _, file := range inventory.Files {
		suites, tests, err := r.saveTestFile(ctx, queries, analysisID, file, 0)
		if err != nil {
			return 0, 0, fmt.Errorf("save test file %s: %w", file.Path, err)
		}
		totalSuites += suites
		totalTests += tests
	}

	return totalSuites, totalTests, nil
}

func (r *AnalysisRepository) saveTestFile(ctx context.Context, queries *db.Queries, analysisID pgtype.UUID, file analysis.TestFile, depth int) (int, int, error) {
	var totalSuites, totalTests int

	for _, suite := range file.Suites {
		suites, tests, err := r.saveSuite(ctx, queries, analysisID, pgtype.UUID{}, file, suite, depth)
		if err != nil {
			return 0, 0, err
		}
		totalSuites += suites
		totalTests += tests
	}

	if len(file.Tests) > 0 {
		implicitSuite, err := r.createImplicitSuite(ctx, queries, analysisID, file, depth)
		if err != nil {
			return 0, 0, err
		}
		totalSuites++

		for _, test := range file.Tests {
			if err := r.saveTest(ctx, queries, implicitSuite.ID, test); err != nil {
				return 0, 0, err
			}
			totalTests++
		}
	}

	return totalSuites, totalTests, nil
}

func (r *AnalysisRepository) createImplicitSuite(ctx context.Context, queries *db.Queries, analysisID pgtype.UUID, file analysis.TestFile, depth int) (db.TestSuite, error) {
	suite, err := queries.CreateTestSuite(ctx, db.CreateTestSuiteParams{
		AnalysisID: analysisID,
		ParentID:   pgtype.UUID{},
		Name:       file.Path,
		FilePath:   file.Path,
		LineNumber: pgtype.Int4{Int32: 1, Valid: true},
		Framework:  pgtype.Text{String: file.Framework, Valid: file.Framework != ""},
		Depth:      int32(depth),
	})
	if err != nil {
		return db.TestSuite{}, fmt.Errorf("create implicit suite: %w", err)
	}
	return suite, nil
}

func (r *AnalysisRepository) saveSuite(ctx context.Context, queries *db.Queries, analysisID, parentID pgtype.UUID, file analysis.TestFile, suite analysis.TestSuite, depth int) (int, int, error) {
	name := truncateString(suite.Name, maxTestSuiteNameLength)

	created, err := queries.CreateTestSuite(ctx, db.CreateTestSuiteParams{
		AnalysisID: analysisID,
		ParentID:   parentID,
		Name:       name,
		FilePath:   file.Path,
		LineNumber: pgtype.Int4{Int32: int32(suite.Location.StartLine), Valid: true},
		Framework:  pgtype.Text{String: file.Framework, Valid: file.Framework != ""},
		Depth:      int32(depth),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("create suite (name=%q, file=%s, line=%d): %w",
			truncateString(suite.Name, 100), file.Path, suite.Location.StartLine, err)
	}

	totalSuites := 1
	var totalTests int

	for _, test := range suite.Tests {
		if err := r.saveTest(ctx, queries, created.ID, test); err != nil {
			return 0, 0, err
		}
		totalTests++
	}

	for _, nested := range suite.Suites {
		suites, tests, err := r.saveSuite(ctx, queries, analysisID, created.ID, file, nested, depth+1)
		if err != nil {
			return 0, 0, err
		}
		totalSuites += suites
		totalTests += tests
	}

	return totalSuites, totalTests, nil
}

const (
	maxTestCaseNameLength  = 2000
	maxTestSuiteNameLength = 500
)

func (r *AnalysisRepository) saveTest(ctx context.Context, queries *db.Queries, suiteID pgtype.UUID, test analysis.Test) error {
	status := mapTestStatus(test.Status)
	name := truncateString(test.Name, maxTestCaseNameLength)

	_, err := queries.CreateTestCase(ctx, db.CreateTestCaseParams{
		SuiteID:    suiteID,
		Name:       name,
		LineNumber: pgtype.Int4{Int32: int32(test.Location.StartLine), Valid: true},
		Status:     status,
		Tags:       []byte("[]"),
	})
	if err != nil {
		return fmt.Errorf("create test case (name=%q, line=%d): %w",
			truncateString(test.Name, 100), test.Location.StartLine, err)
	}
	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func mapTestStatus(status analysis.TestStatus) db.TestStatus {
	switch status {
	case analysis.TestStatusSkipped:
		return db.TestStatusSkipped
	case analysis.TestStatusTodo:
		return db.TestStatusTodo
	default:
		return db.TestStatusActive
	}
}
