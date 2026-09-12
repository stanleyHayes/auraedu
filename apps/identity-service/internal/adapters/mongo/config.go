package mongo

import (
	"context"
	"errors"
	"os"

	pm "github.com/auraedu/platform/mongo"
)

// OpenFromEnv initializes the service's isolated database and its indexes.
func OpenFromEnv(ctx context.Context) (*Repository, *pm.Store, error) {
	database := os.Getenv("MONGODB_DATABASE")
	if database == "" {
		database = "auraedu_identity"
	}
	store, err := pm.Open(ctx, pm.Config{URI: os.Getenv("MONGODB_URI"), Database: database, MaxPoolSize: 2})
	if err != nil {
		return nil, nil, err
	}
	repo := NewRepository(store)
	if err = repo.EnsureIndexes(ctx); err != nil {
		return nil, nil, errors.Join(err, store.Close(ctx))
	}
	return repo, store, nil
}
