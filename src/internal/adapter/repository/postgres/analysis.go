package postgres

import (
	"context"
	"encoding/json"
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
	"github.com/specvital/core/pkg/parser"
	"github.com/specvital/worker/internal/domain/analysis"
	"github.com/specvital/worker/internal/infra/db"
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
	Branch         string
	CommitSHA      string
	ExternalRepoID string
	Owner          string
	ParserVersion  string
	Repo           string
	Result         *parser.ScanResult
}

func (p SaveAnalysisResultParams) Validate() error {
	domainParams := analysis.CreateAnalysisRecordParams{
		Branch:         p.Branch,
		CommitSHA:      p.CommitSHA,
		ExternalRepoID: p.ExternalRepoID,
		Owner:          p.Owner,
		ParserVersion:  p.ParserVersion,
		Repo:           p.Repo,
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

	var codebaseID pgtype.UUID
	if params.CodebaseID != nil {
		codebaseID = toPgUUID(*params.CodebaseID)
	} else {
		codebase, upsertErr := queries.UpsertCodebase(ctx, db.UpsertCodebaseParams{
			Host:           defaultHost,
			Owner:          params.Owner,
			Name:           params.Repo,
			DefaultBranch:  pgtype.Text{String: params.Branch, Valid: params.Branch != ""},
			ExternalRepoID: params.ExternalRepoID,
		})
		if upsertErr != nil {
			return analysis.NilUUID, fmt.Errorf("upsert codebase: %w", upsertErr)
		}
		codebaseID = codebase.ID
	}

	analysisID := analysis.NewUUID()
	if params.AnalysisID != nil {
		analysisID = *params.AnalysisID
	}

	dbAnalysis, err := queries.CreateAnalysis(ctx, db.CreateAnalysisParams{
		ID:            toPgUUID(analysisID),
		CodebaseID:    codebaseID,
		CommitSha:     params.CommitSHA,
		BranchName:    pgtype.Text{String: params.Branch, Valid: params.Branch != ""},
		Status:        db.AnalysisStatusRunning,
		StartedAt:     pgtype.Timestamptz{Time: startedAt, Valid: true},
		ParserVersion: params.ParserVersion,
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

	_, totalSuites, totalTests, err := r.saveInventory(ctx, tx, pgID, params.Inventory)
	if err != nil {
		return fmt.Errorf("save inventory: %w", err)
	}

	if err := queries.UpdateAnalysisCompleted(ctx, db.UpdateAnalysisCompletedParams{
		ID:          pgID,
		TotalSuites: int32(totalSuites),
		TotalTests:  int32(totalTests),
		CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CommittedAt: pgtype.Timestamptz{Time: params.CommittedAt, Valid: !params.CommittedAt.IsZero()},
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return analysis.ErrAlreadyCompleted
		}
		return fmt.Errorf("update analysis: %w", err)
	}

	if params.UserID != nil {
		userUUID, parseErr := analysis.ParseUUID(*params.UserID)
		if parseErr != nil {
			slog.WarnContext(ctx, "invalid user ID format, skipping history record",
				"user_id", *params.UserID,
				"error", parseErr,
			)
		} else {
			if err := queries.RecordUserAnalysisHistory(ctx, db.RecordUserAnalysisHistoryParams{
				UserID:     toPgUUID(userUUID),
				AnalysisID: pgID,
			}); err != nil {
				return fmt.Errorf("record user analysis history: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// SaveAnalysisResult is a convenience method that combines CreateAnalysisRecord and SaveAnalysisInventory
// in a single transaction. This method is kept for backward compatibility with existing code that uses
// parser.ScanResult. It is not part of the domain interface.
func (r *AnalysisRepository) SaveAnalysisResult(ctx context.Context, params SaveAnalysisResultParams) error {
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
				"operation", "SaveAnalysisResult",
				"error", rbErr,
				"owner", params.Owner,
				"repo", params.Repo,
			)
		}
	}()

	queries := db.New(tx)
	startedAt := time.Now()

	codebase, err := queries.UpsertCodebase(ctx, db.UpsertCodebaseParams{
		Host:           defaultHost,
		Owner:          params.Owner,
		Name:           params.Repo,
		DefaultBranch:  pgtype.Text{String: params.Branch, Valid: params.Branch != ""},
		ExternalRepoID: params.ExternalRepoID,
	})
	if err != nil {
		return fmt.Errorf("upsert codebase: %w", err)
	}

	analysisID := analysis.NewUUID()
	dbAnalysis, err := queries.CreateAnalysis(ctx, db.CreateAnalysisParams{
		ID:            toPgUUID(analysisID),
		CodebaseID:    codebase.ID,
		CommitSha:     params.CommitSHA,
		BranchName:    pgtype.Text{String: params.Branch, Valid: params.Branch != ""},
		Status:        db.AnalysisStatusRunning,
		StartedAt:     pgtype.Timestamptz{Time: startedAt, Valid: true},
		ParserVersion: params.ParserVersion,
	})
	if err != nil {
		return fmt.Errorf("create analysis: %w", err)
	}

	domainInventory := convertCoreToDomainInventory(params.Result.Inventory)
	pgID := dbAnalysis.ID

	_, totalSuites, totalTests, err := r.saveInventory(ctx, tx, pgID, domainInventory)
	if err != nil {
		return fmt.Errorf("save inventory: %w", err)
	}

	if err := queries.UpdateAnalysisCompleted(ctx, db.UpdateAnalysisCompletedParams{
		ID:          pgID,
		TotalSuites: int32(totalSuites),
		TotalTests:  int32(totalTests),
		CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CommittedAt: pgtype.Timestamptz{},
	}); err != nil {
		return fmt.Errorf("update analysis: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
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
	case analysis.TestStatusFocused:
		return db.TestStatusFocused
	case analysis.TestStatusSkipped:
		return db.TestStatusSkipped
	case analysis.TestStatusTodo:
		return db.TestStatusTodo
	case analysis.TestStatusXfail:
		return db.TestStatusXfail
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
			LastParserVersion:   row.LastParserVersion,
			LastViewedAt:        row.LastViewedAt.Time,
			Name:                row.Name,
			Owner:               row.Owner,
		}

		if row.LastCompletedAt.Valid {
			t := row.LastCompletedAt.Time
			info.LastCompletedAt = &t
		}

		if row.LastCommitSha.Valid {
			info.LastCommitSHA = row.LastCommitSha.String
		}

		result = append(result, info)
	}

	return result, nil
}

type flatSuite struct {
	tempID     int
	parentTemp int // -1 if root
	suite      analysis.TestSuite
	fileID     pgtype.UUID
	depth      int
}

type flatTest struct {
	suiteTempID int
	test        analysis.Test
}

func flattenInventory(inventory *analysis.Inventory, fileIDs map[string]pgtype.UUID) ([]flatSuite, []flatTest) {
	if inventory == nil {
		return nil, nil
	}

	var suites []flatSuite
	var tests []flatTest
	tempID := 0

	for _, file := range inventory.Files {
		fileID := fileIDs[file.Path]

		for _, suite := range file.Suites {
			flattenSuiteRecursive(&suites, &tests, &tempID, -1, fileID, suite, 0)
		}

		if len(file.Tests) > 0 {
			implicitSuite := flatSuite{
				tempID:     tempID,
				parentTemp: -1,
				suite: analysis.TestSuite{
					Name:     file.Path,
					Location: analysis.Location{StartLine: 1},
				},
				fileID: fileID,
				depth:  0,
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

func flattenSuiteRecursive(suites *[]flatSuite, tests *[]flatTest, tempID *int, parentTemp int, fileID pgtype.UUID, suite analysis.TestSuite, depth int) {
	currentTempID := *tempID
	*suites = append(*suites, flatSuite{
		tempID:     currentTempID,
		parentTemp: parentTemp,
		suite:      suite,
		fileID:     fileID,
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
		flattenSuiteRecursive(suites, tests, tempID, currentTempID, fileID, nested, depth+1)
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
			s.fileID,
			parentID,
			truncateString(s.suite.Name, maxTestSuiteNameLength),
			pgtype.Int4{Int32: int32(s.suite.Location.StartLine), Valid: true},
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

func (r *AnalysisRepository) saveFiles(
	ctx context.Context,
	tx pgx.Tx,
	analysisID pgtype.UUID,
	files []analysis.TestFile,
) (map[string]pgtype.UUID, error) {
	fileIDs := make(map[string]pgtype.UUID, len(files))
	queries := db.New(tx)

	for _, file := range files {
		var hintsJSON []byte
		if file.DomainHints != nil {
			var err error
			hintsJSON, err = json.Marshal(file.DomainHints)
			if err != nil {
				return nil, fmt.Errorf("marshal domain hints for %q: %w", file.Path, err)
			}
		}

		fileID, err := queries.InsertTestFile(ctx, db.InsertTestFileParams{
			AnalysisID:  analysisID,
			FilePath:    file.Path,
			Framework:   pgtype.Text{String: file.Framework, Valid: file.Framework != ""},
			DomainHints: hintsJSON,
		})
		if err != nil {
			return nil, fmt.Errorf("insert test file %q: %w", file.Path, err)
		}
		fileIDs[file.Path] = fileID
	}

	return fileIDs, nil
}

func (r *AnalysisRepository) saveInventory(
	ctx context.Context,
	tx pgx.Tx,
	analysisID pgtype.UUID,
	inventory *analysis.Inventory,
) (totalFiles, totalSuites, totalTests int, err error) {
	if inventory == nil || len(inventory.Files) == 0 {
		return 0, 0, 0, nil
	}

	fileIDs, err := r.saveFiles(ctx, tx, analysisID, inventory.Files)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("save files: %w", err)
	}

	suites, tests := flattenInventory(inventory, fileIDs)
	if len(suites) == 0 {
		return len(inventory.Files), 0, 0, nil
	}

	suitesByDepth := groupByDepth(suites)
	maxDepth := maxDepthInSuites(suitesByDepth)

	allIDs := make(map[int]pgtype.UUID)
	for depth := 0; depth <= maxDepth; depth++ {
		if err := ctx.Err(); err != nil {
			return 0, 0, 0, err
		}
		depthSuites := suitesByDepth[depth]
		if len(depthSuites) == 0 {
			continue
		}
		newIDs, err := r.saveSuitesBatch(ctx, tx, depthSuites, allIDs)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("save suites at depth %d (count=%d): %w", depth, len(depthSuites), err)
		}
		maps.Copy(allIDs, newIDs)
	}

	if err := r.saveTestsCopyFrom(ctx, tx, tests, allIDs); err != nil {
		return 0, 0, 0, err
	}

	return len(inventory.Files), len(suites), len(tests), nil
}
