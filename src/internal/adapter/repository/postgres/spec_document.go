package postgres

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/worker/internal/domain/analysis"
	"github.com/specvital/worker/internal/domain/specview"
	"github.com/specvital/worker/internal/infra/db"
)

var _ specview.Repository = (*SpecDocumentRepository)(nil)

type SpecDocumentRepository struct {
	pool *pgxpool.Pool
}

type suiteInfo struct {
	depth    int32
	name     string
	parentID pgtype.UUID
}

func NewSpecDocumentRepository(pool *pgxpool.Pool) *SpecDocumentRepository {
	return &SpecDocumentRepository{pool: pool}
}

func (r *SpecDocumentRepository) FindDocumentByContentHash(
	ctx context.Context,
	contentHash []byte,
	language specview.Language,
	modelID string,
) (*specview.SpecDocument, error) {
	queries := db.New(r.pool)

	doc, err := queries.FindSpecDocumentByContentHash(ctx, db.FindSpecDocumentByContentHashParams{
		ContentHash: contentHash,
		Language:    string(language),
		ModelID:     modelID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find spec document: %w", err)
	}

	return &specview.SpecDocument{
		AnalysisID:  fromPgUUID(doc.AnalysisID).String(),
		ContentHash: doc.ContentHash,
		CreatedAt:   doc.CreatedAt.Time,
		ID:          fromPgUUID(doc.ID).String(),
		Language:    specview.Language(doc.Language),
		ModelID:     doc.ModelID,
	}, nil
}

func (r *SpecDocumentRepository) GetTestDataByAnalysisID(
	ctx context.Context,
	analysisID string,
) ([]specview.FileInfo, error) {
	parsedID, err := analysis.ParseUUID(analysisID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid analysis ID format", specview.ErrInvalidInput)
	}

	queries := db.New(r.pool)

	exists, err := queries.CheckAnalysisExists(ctx, toPgUUID(parsedID))
	if err != nil {
		return nil, fmt.Errorf("check analysis exists: %w", err)
	}
	if !exists {
		return nil, specview.ErrAnalysisNotFound
	}

	rows, err := queries.GetTestDataByAnalysisID(ctx, toPgUUID(parsedID))
	if err != nil {
		return nil, fmt.Errorf("get test data: %w", err)
	}

	return r.aggregateTestData(rows)
}

func (r *SpecDocumentRepository) aggregateTestData(
	rows []db.GetTestDataByAnalysisIDRow,
) ([]specview.FileInfo, error) {
	fileMap := make(map[string]*specview.FileInfo)
	suiteMap := make(map[string]suiteInfo)
	testIndex := 0

	for _, row := range rows {
		file, exists := fileMap[row.FilePath]
		if !exists {
			file = &specview.FileInfo{
				Framework: row.Framework.String,
				Path:      row.FilePath,
				Tests:     make([]specview.TestInfo, 0),
			}

			if row.DomainHints != nil {
				var hints specview.DomainHints
				if err := json.Unmarshal(row.DomainHints, &hints); err != nil {
					return nil, fmt.Errorf("unmarshal domain hints for file %q: %w", row.FilePath, err)
				}
				file.DomainHints = &hints
			}

			fileMap[row.FilePath] = file
		}

		suiteIDStr := uuidBytesToString(row.SuiteID.Bytes)
		if _, exists := suiteMap[suiteIDStr]; !exists {
			suiteMap[suiteIDStr] = suiteInfo{
				depth:    row.SuiteDepth,
				name:     row.SuiteName,
				parentID: row.SuiteParentID,
			}
		}

		suitePath := r.buildSuitePath(row.SuiteID, suiteMap)

		file.Tests = append(file.Tests, specview.TestInfo{
			Index:      testIndex,
			Name:       row.TestName,
			SuitePath:  suitePath,
			TestCaseID: fromPgUUID(row.TestCaseID).String(),
		})
		testIndex++
	}

	result := make([]specview.FileInfo, 0, len(fileMap))
	for _, file := range fileMap {
		result = append(result, *file)
	}

	slices.SortFunc(result, func(a, b specview.FileInfo) int {
		return cmp.Compare(a.Path, b.Path)
	})

	return result, nil
}

func (r *SpecDocumentRepository) buildSuitePath(
	suiteID pgtype.UUID,
	suiteMap map[string]suiteInfo,
) string {
	var pathParts []string
	currentID := suiteID

	for currentID.Valid {
		idStr := uuidBytesToString(currentID.Bytes)
		info, exists := suiteMap[idStr]
		if !exists {
			break
		}

		pathParts = append(pathParts, info.name)
		currentID = info.parentID
	}

	if len(pathParts) == 0 {
		return ""
	}

	slices.Reverse(pathParts)
	return strings.Join(pathParts, " > ")
}

func (r *SpecDocumentRepository) SaveDocument(
	ctx context.Context,
	doc *specview.SpecDocument,
) error {
	if doc == nil {
		return fmt.Errorf("%w: document is nil", specview.ErrInvalidInput)
	}
	if len(doc.ContentHash) == 0 {
		return fmt.Errorf("%w: content hash is required", specview.ErrInvalidInput)
	}
	if doc.Language == "" {
		return fmt.Errorf("%w: language is required", specview.ErrInvalidInput)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			slog.ErrorContext(ctx, "failed to rollback transaction",
				"operation", "SaveDocument",
				"analysis_id", doc.AnalysisID,
				"error", rbErr,
			)
		}
	}()

	queries := db.New(tx)

	analysisID, err := analysis.ParseUUID(doc.AnalysisID)
	if err != nil {
		return fmt.Errorf("%w: invalid analysis ID", specview.ErrInvalidInput)
	}

	docID, err := queries.InsertSpecDocument(ctx, db.InsertSpecDocumentParams{
		AnalysisID:  toPgUUID(analysisID),
		ContentHash: doc.ContentHash,
		Language:    string(doc.Language),
		ModelID:     doc.ModelID,
	})
	if err != nil {
		return fmt.Errorf("insert spec document: %w", err)
	}

	doc.ID = fromPgUUID(docID).String()

	if err := r.saveDomains(ctx, tx, docID, doc.Domains); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *SpecDocumentRepository) saveDomains(
	ctx context.Context,
	tx pgx.Tx,
	documentID pgtype.UUID,
	domains []specview.Domain,
) error {
	if len(domains) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for i, domain := range domains {
		batch.Queue(db.InsertSpecDomainBatch,
			documentID,
			domain.Name,
			pgtype.Text{String: domain.Description, Valid: domain.Description != ""},
			int32(i),
			confidenceToNumeric(domain.Confidence),
		)
	}

	results := tx.SendBatch(ctx, batch)
	defer results.Close()

	domainIDs := make([]pgtype.UUID, len(domains))
	for i := range domains {
		var id pgtype.UUID
		if err := results.QueryRow().Scan(&id); err != nil {
			return fmt.Errorf("scan domain ID for %q: %w", domains[i].Name, err)
		}
		domainIDs[i] = id
		domains[i].ID = fromPgUUID(id).String()
	}

	if err := results.Close(); err != nil {
		return fmt.Errorf("close domain batch: %w", err)
	}

	for i, domain := range domains {
		if err := r.saveFeatures(ctx, tx, domainIDs[i], domain.Features); err != nil {
			return fmt.Errorf("save features for domain %q: %w", domain.Name, err)
		}
	}

	return nil
}

func (r *SpecDocumentRepository) saveFeatures(
	ctx context.Context,
	tx pgx.Tx,
	domainID pgtype.UUID,
	features []specview.Feature,
) error {
	if len(features) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for i, feature := range features {
		batch.Queue(db.InsertSpecFeatureBatch,
			domainID,
			feature.Name,
			pgtype.Text{String: feature.Description, Valid: feature.Description != ""},
			int32(i),
		)
	}

	results := tx.SendBatch(ctx, batch)
	defer results.Close()

	featureIDs := make([]pgtype.UUID, len(features))
	for i := range features {
		var id pgtype.UUID
		if err := results.QueryRow().Scan(&id); err != nil {
			return fmt.Errorf("scan feature ID for %q: %w", features[i].Name, err)
		}
		featureIDs[i] = id
		features[i].ID = fromPgUUID(id).String()
	}

	if err := results.Close(); err != nil {
		return fmt.Errorf("close feature batch: %w", err)
	}

	var allBehaviors []behaviorWithFeatureID
	for i, feature := range features {
		for j, behavior := range feature.Behaviors {
			allBehaviors = append(allBehaviors, behaviorWithFeatureID{
				behavior:  behavior,
				featureID: featureIDs[i],
				sortOrder: j,
			})
		}
	}

	if len(allBehaviors) > 0 {
		if err := r.saveBehaviorsCopyFrom(ctx, tx, allBehaviors); err != nil {
			return err
		}
	}

	return nil
}

type behaviorWithFeatureID struct {
	behavior  specview.Behavior
	featureID pgtype.UUID
	sortOrder int
}

func (r *SpecDocumentRepository) saveBehaviorsCopyFrom(
	ctx context.Context,
	tx pgx.Tx,
	behaviors []behaviorWithFeatureID,
) error {
	rows := make([][]any, len(behaviors))

	for i, b := range behaviors {
		var testCaseID pgtype.UUID
		if b.behavior.TestCaseID != "" {
			parsedID, err := analysis.ParseUUID(b.behavior.TestCaseID)
			if err != nil {
				return fmt.Errorf("parse test case ID %q: %w", b.behavior.TestCaseID, err)
			}
			testCaseID = toPgUUID(parsedID)
		}

		rows[i] = []any{
			b.featureID,
			testCaseID,
			b.behavior.OriginalName,
			b.behavior.Description,
			int32(b.sortOrder),
		}
	}

	_, err := tx.Conn().CopyFrom(
		ctx,
		pgx.Identifier{"spec_behaviors"},
		db.SpecBehaviorCopyColumns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("copy spec behaviors: %w", err)
	}

	return nil
}

func uuidBytesToString(bytes [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func confidenceToNumeric(confidence float64) pgtype.Numeric {
	intVal := int64(confidence * 100)
	return pgtype.Numeric{
		Int:   big.NewInt(intVal),
		Exp:   -2,
		Valid: true,
	}
}
