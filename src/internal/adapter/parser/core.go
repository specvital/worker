package parser

import (
	"context"
	"fmt"

	"github.com/specvital/collector/internal/adapter/mapping"
	"github.com/specvital/collector/internal/domain/analysis"
	"github.com/specvital/core/pkg/parser"
	"github.com/specvital/core/pkg/source"
)

// CoreParser implements analysis.Parser using specvital/core's parser package.
type CoreParser struct{}

// NewCoreParser creates a new CoreParser.
func NewCoreParser() *CoreParser {
	return &CoreParser{}
}

// coreSourceProvider is implemented by sources that can provide
// the underlying source.Source for the core parser.
type coreSourceProvider interface {
	CoreSource() source.Source
}

// Scan implements analysis.Parser by delegating to the core parser
// and converting the result to domain types.
func (p *CoreParser) Scan(ctx context.Context, src analysis.Source) (*analysis.Inventory, error) {
	provider, ok := src.(coreSourceProvider)
	if !ok {
		return nil, fmt.Errorf("source does not implement coreSourceProvider interface")
	}

	result, err := parser.Scan(ctx, provider.CoreSource())
	if err != nil {
		return nil, fmt.Errorf("core parser scan: %w", err)
	}

	return mapping.ConvertCoreToDomainInventory(result.Inventory), nil
}
