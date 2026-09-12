package mongo

import (
	"context"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func closeCursor(ctx context.Context, cursor *mongo.Cursor) {
	if err := cursor.Close(ctx); err != nil {
		slog.Warn("close MongoDB cursor", "error", err)
	}
}
