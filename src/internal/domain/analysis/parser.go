package analysis

import "context"

type Parser interface {
	Scan(ctx context.Context, src Source) (*Inventory, error)
}
