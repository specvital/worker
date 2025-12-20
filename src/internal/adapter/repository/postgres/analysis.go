package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/specvital/collector/internal/infra/db"
	"github.com/specvital/core/pkg/parser"
)

var _ analysis.AutoRefreshRepository = (*AnalysisRepository)(nil)

const defaultHost = "github.com"
const maxErrorMessageLength = 1000

func truncateErrorMessage(msg string) string {
	if len(msg) <= maxErrorMessageLength {
		return msg
	}
	return msg[:maxErrorMessageLength-15] + "... (truncated)"
}

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

type AnalysisRepository struct {
	pool *pgxpool.Pool
}

func NewAnalysisRepository(pool *pgxpool.Pool) *AnalysisRepository {
	return &AnalysisRepository{pool: pool}
}

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

	analysisID := analysis.NewUUID()
	if params.AnalysisID != nil {
		analysisID = *params.AnalysisID
	}

	dbAnalysis, err := queries.CreateAnalysis(ctx, db.CreateAnalysisParams{
		ID:         toPgUUID(analysisID),
		CodebaseID: codebase.ID,
		CommitSha:  params.CommitSHA,
		BranchName: pgtype.Text{String: params.Branch, Valid: params.Branch != ""},
		Status:     db.AnalysisStatusRunning,
		StartedAt:  pgtype.Timestamptz{Time: startedAt, Valid: true},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return analysis.NilUUID, fmt.Errorf("%w: analysis ID already exists", analysis.ErrInvalidInput)
		}
		return analysis.NilUUID, fmt.Errorf("create analysis: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return analysis.NilUUID, fmt.Errorf("commit transaction: %w", err)
	}

	return fromPgUUID(dbAnalysis.ID), nil
}

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

	totalSuites, totalTests, err := r.saveInventory(ctx, tx, pgID, params.Inventory)
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

const (
	maxTestCaseNameLength  = 2000
	maxTestSuiteNameLength = 500
)

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	truncated := s[:maxLen-3]
	// Ensure we don't break UTF-8 encoding by removing incomplete runes
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "..."
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

func (r *AnalysisRepository) GetCodebasesForAutoRefresh(ctx context.Context) ([]analysis.CodebaseRefreshInfo, error) {
	queries := db.New(r.pool)

	rows, err := queries.GetCodebasesForAutoRefresh(ctx)
	if err != nil {
		return nil, fmt.Errorf("query codebases for auto-refresh: %w", err)
	}

	result := make([]analysis.CodebaseRefreshInfo, 0, len(rows))
	for _, row := range rows {
		info := analysis.CodebaseRefreshInfo{
			ConsecutiveFailures: int(row.ConsecutiveFailures),
			Host:                row.Host,
			ID:                  fromPgUUID(row.ID),
			LastViewedAt:        row.LastViewedAt.Time,
			Name:                row.Name,
			Owner:               row.Owner,
		}

		if row.LastCompletedAt.Valid {
			t := row.LastCompletedAt.Time
			info.LastCompletedAt = &t
		}

		result = append(result, info)
	}

	return result, nil
}

type flatSuite struct {
	tempID     int
	parentTemp int // -1 if root
	suite      analysis.TestSuite
	file       analysis.TestFile
	depth      int
}

type flatTest struct {
	suiteTempID int
	test        analysis.Test
}

func flattenInventory(inventory *analysis.Inventory) ([]flatSuite, []flatTest) {
	if inventory == nil {
		return nil, nil
	}

	var suites []flatSuite
	var tests []flatTest
	tempID := 0

	for _, file := range inventory.Files {
		for _, suite := range file.Suites {
			flattenSuiteRecursive(&suites, &tests, &tempID, -1, file, suite, 0)
		}

		if len(file.Tests) > 0 {
			implicitSuite := flatSuite{
				tempID:     tempID,
				parentTemp: -1,
				suite: analysis.TestSuite{
					Name:     file.Path,
					Location: analysis.Location{StartLine: 1},
				},
				file:  file,
				depth: 0,
			}
			suites = append(suites, implicitSuite)

			for _, test := range file.Tests {
				tests = append(tests, flatTest{
					suiteTempID: tempID,
					test:        test,
				})
			}
			tempID++
		}
	}

	return suites, tests
}

func flattenSuiteRecursive(suites *[]flatSuite, tests *[]flatTest, tempID *int, parentTemp int, file analysis.TestFile, suite analysis.TestSuite, depth int) {
	currentTempID := *tempID
	*suites = append(*suites, flatSuite{
		tempID:     currentTempID,
		parentTemp: parentTemp,
		suite:      suite,
		file:       file,
		depth:      depth,
	})
	*tempID++

	for _, test := range suite.Tests {
		*tests = append(*tests, flatTest{
			suiteTempID: currentTempID,
			test:        test,
		})
	}

	for _, nested := range suite.Suites {
		flattenSuiteRecursive(suites, tests, tempID, currentTempID, file, nested, depth+1)
	}
}

func groupByDepth(suites []flatSuite) map[int][]flatSuite {
	result := make(map[int][]flatSuite)
	for _, s := range suites {
		result[s.depth] = append(result[s.depth], s)
	}
	return result
}

func maxDepthInSuites(suitesByDepth map[int][]flatSuite) int {
	if len(suitesByDepth) == 0 {
		return -1
	}
	depths := slices.Collect(maps.Keys(suitesByDepth))
	return slices.Max(depths)
}

func (r *AnalysisRepository) saveSuitesBatch(
	ctx context.Context,
	tx pgx.Tx,
	analysisID pgtype.UUID,
	suites []flatSuite,
	parentIDs map[int]pgtype.UUID,
) (map[int]pgtype.UUID, error) {
	if len(suites) == 0 {
		return make(map[int]pgtype.UUID), nil
	}

	batch := &pgx.Batch{}

	for _, s := range suites {
		parentID := pgtype.UUID{}
		if s.parentTemp >= 0 {
			parentID = parentIDs[s.parentTemp]
		}

		batch.Queue(db.InsertTestSuiteBatch,
			analysisID,
			parentID,
			truncateString(s.suite.Name, maxTestSuiteNameLength),
			s.file.Path,
			pgtype.Int4{Int32: int32(s.suite.Location.StartLine), Valid: true},
			pgtype.Text{String: s.file.Framework, Valid: s.file.Framework != ""},
			int32(s.depth),
		)
	}

	results := tx.SendBatch(ctx, batch)
	defer results.Close()

	newIDs := make(map[int]pgtype.UUID)
	for _, s := range suites {
		var id pgtype.UUID
		if err := results.QueryRow().Scan(&id); err != nil {
			return nil, fmt.Errorf("scan suite ID for %q: %w", truncateString(s.suite.Name, 50), err)
		}
		newIDs[s.tempID] = id
	}
	return newIDs, nil
}

func (r *AnalysisRepository) saveTestsCopyFrom(
	ctx context.Context,
	tx pgx.Tx,
	tests []flatTest,
	suiteIDs map[int]pgtype.UUID,
) error {
	if len(tests) == 0 {
		return nil
	}

	for _, t := range tests {
		if _, exists := suiteIDs[t.suiteTempID]; !exists {
			return fmt.Errorf("suite ID not found for tempID %d (test=%q)",
				t.suiteTempID, truncateString(t.test.Name, 50))
		}
	}

	rows := make([][]any, len(tests))
	for i, t := range tests {
		rows[i] = []any{
			suiteIDs[t.suiteTempID],
			truncateString(t.test.Name, maxTestCaseNameLength),
			pgtype.Int4{Int32: int32(t.test.Location.StartLine), Valid: true},
			mapTestStatus(t.test.Status),
			[]byte("[]"),
			pgtype.Text{},
		}
	}

	_, err := tx.Conn().CopyFrom(
		ctx,
		pgx.Identifier{"test_cases"},
		db.TestCaseCopyColumns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("copy test cases: %w", err)
	}
	return nil
}

func (r *AnalysisRepository) saveInventory(
	ctx context.Context,
	tx pgx.Tx,
	analysisID pgtype.UUID,
	inventory *analysis.Inventory,
) (totalSuites, totalTests int, err error) {
	if inventory == nil {
		return 0, 0, nil
	}

	suites, tests := flattenInventory(inventory)
	if len(suites) == 0 {
		return 0, 0, nil
	}

	suitesByDepth := groupByDepth(suites)
	maxDepth := maxDepthInSuites(suitesByDepth)

	allIDs := make(map[int]pgtype.UUID)
	for depth := 0; depth <= maxDepth; depth++ {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		depthSuites := suitesByDepth[depth]
		if len(depthSuites) == 0 {
			continue
		}
		newIDs, err := r.saveSuitesBatch(ctx, tx, analysisID, depthSuites, allIDs)
		if err != nil {
			return 0, 0, fmt.Errorf("save suites at depth %d (count=%d): %w", depth, len(depthSuites), err)
		}
		maps.Copy(allIDs, newIDs)
	}

	if err := r.saveTestsCopyFrom(ctx, tx, tests, allIDs); err != nil {
		return 0, 0, err
	}

	return len(suites), len(tests), nil
}
