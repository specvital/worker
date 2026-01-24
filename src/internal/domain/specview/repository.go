package specview

import "context"

// Repository defines the interface for spec-view document persistence.
type Repository interface {
	// FindDocumentByContentHash checks if a document already exists with the given hash for the user.
	// Returns nil without error if no document is found.
	FindDocumentByContentHash(ctx context.Context, userID string, contentHash []byte, language Language, modelID string) (*SpecDocument, error)

	// GetAnalysisContext retrieves repository context (host, owner, repo) for an analysis.
	// Returns ErrAnalysisNotFound if the analysis does not exist.
	GetAnalysisContext(ctx context.Context, analysisID string) (*AnalysisContext, error)

	// GetTestDataByAnalysisID retrieves test inventory for spec-view generation.
	// Returns ErrAnalysisNotFound if the analysis does not exist.
	GetTestDataByAnalysisID(ctx context.Context, analysisID string) ([]FileInfo, error)

	// RecordUsageEvent records a usage event for quota tracking.
	// Only called on cache miss (when AI processing runs).
	// quotaAmount is the number of test cases processed.
	RecordUsageEvent(ctx context.Context, userID string, documentID string, quotaAmount int) error

	// RecordUserHistory records a user's specview generation to user_specview_history.
	// Uses UPSERT: creates new record or updates updated_at on re-request.
	RecordUserHistory(ctx context.Context, userID string, documentID string) error

	// SaveDocument saves the complete 4-table hierarchy in a single transaction.
	// This includes spec_documents, spec_domains, spec_features, and spec_behaviors.
	SaveDocument(ctx context.Context, doc *SpecDocument) error

	// FindCachedBehaviors looks up cached behavior descriptions by cache key hashes.
	// Returns a map of cache_key_hash (hex-encoded) -> converted_description.
	// Only found entries are included in the result map.
	FindCachedBehaviors(ctx context.Context, cacheKeyHashes [][]byte) (map[string]string, error)

	// SaveBehaviorCache saves behavior cache entries to the database.
	// Uses upsert semantics: existing entries are updated, new entries are inserted.
	SaveBehaviorCache(ctx context.Context, entries []BehaviorCacheEntry) error
}
