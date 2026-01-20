package specview

import "context"

// AIProvider defines the interface for AI-based spec-view generation.
type AIProvider interface {
	// ClassifyDomains performs Phase 1: domain and feature classification.
	// Groups tests into domains and features based on their names and paths.
	// Returns token usage for the API call.
	ClassifyDomains(ctx context.Context, input Phase1Input) (*Phase1Output, *TokenUsage, error)

	// ConvertTestNames performs Phase 2: test name to behavior conversion.
	// Converts technical test names into human-readable behavior descriptions.
	// Returns token usage for the API call.
	ConvertTestNames(ctx context.Context, input Phase2Input) (*Phase2Output, *TokenUsage, error)

	// Close releases resources held by the provider.
	Close() error
}
