package mongo

import (
	"context"

	pmongo "github.com/auraedu/platform/mongo"
)

// Parent version writes inside this transaction fence concurrent deletion.
func inTransaction(ctx context.Context, s *pmongo.Store, fn func(context.Context) error) error {
	return mapped(s.WithTransaction(ctx, fn))
}
