package specview

import "context"

// Repository defines the interface for spec-view document persistence.
type Repository interface {
	// FindDocumentByContentHash checks if a document already exists with the given hash.
	// Returns nil without error if no document is found.
	FindDocumentByContentHash(ctx context.Context, contentHash []byte, language Language, modelID string) (*SpecDocument, error)

	// GetTestDataByAnalysisID retrieves test inventory for spec-view generation.
	// Returns ErrAnalysisNotFound if the analysis does not exist.
	GetTestDataByAnalysisID(ctx context.Context, analysisID string) ([]FileInfo, error)

	// SaveDocument saves the complete 4-table hierarchy in a single transaction.
	// This includes spec_documents, spec_domains, spec_features, and spec_behaviors.
	SaveDocument(ctx context.Context, doc *SpecDocument) error
}
