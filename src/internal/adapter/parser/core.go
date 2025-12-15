package parser

import (
	"context"
	"fmt"

	"github.com/specvital/collector/internal/adapter/mapping"
	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/specvital/core/pkg/parser"
	"github.com/specvital/core/pkg/source"
)

// gitSourceUnwrapper is an internal interface to access the underlying source.Source
// needed by parser.Scan. This avoids exposing the core package types in the domain layer.
//
// Only gitSourceAdapter from the vcs package implements this interface.
// This coupling is acceptable as both adapters wrap the same external library.
type gitSourceUnwrapper interface {
	unwrapGitSource() *source.GitSource
}

// CoreParser implements analysis.Parser using specvital/core's parser package.
type CoreParser struct{}

// NewCoreParser creates a new CoreParser.
func NewCoreParser() *CoreParser {
	return &CoreParser{}
}

// Scan implements analysis.Parser by delegating to the core parser
// and converting the result to domain types.
func (p *CoreParser) Scan(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
	unwrapper, ok := src.(gitSourceUnwrapper)
	if !ok {
		return nil, fmt.Errorf("source does not support unwrapping to core source type")
	}

	gitSrc := unwrapper.unwrapGitSource()

	result, err := parser.Scan(ctx, gitSrc)
	if err != nil {
		return nil, fmt.Errorf("core parser scan: %w", err)
	}

	return mapping.ConvertCoreToDomainInventory(result.Inventory), nil
}
