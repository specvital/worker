package analysis

import "context"

type Parser interface {
	Scan(ctx context.Context, src Source) (*Inventory, error)
}

// StreamingParser provides file-by-file streaming interface for memory-efficient parsing.
type StreamingParser interface {
	Parser
	ScanStream(ctx context.Context, src Source) (<-chan FileResult, error)
}

// FileResult represents a single file parsing result from streaming parser.
type FileResult struct {
	Err  error
	File *TestFile
}
