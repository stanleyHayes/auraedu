package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// RequireTransactions fails before mutations on standalone MongoDB. Atlas Free
// uses a replica set and supports these transactions; standalone fixtures do not.
func (s *Store) RequireTransactions(ctx context.Context) error {
	var hello struct {
		SetName string `bson:"setName"`
		Msg     string `bson:"msg"`
	}
	if err := s.db.RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello); err != nil {
		return fmt.Errorf("mongo: inspect transaction support: %w", err)
	}
	if hello.SetName == "" && hello.Msg != "isdbgrid" {
		return fmt.Errorf("mongo: this operation requires a replica set or mongos (standalone MongoDB is unsupported)")
	}
	return nil
}

// WithTransaction reuses an active transaction belonging to this client. All
// callbacks must propagate their errors and may be retried by the driver.
func (s *Store) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	if session := mongo.SessionFromContext(ctx); session != nil {
		if session.Client() != s.client {
			return fmt.Errorf("mongo: session belongs to another client")
		}
		if session.TransactionRunning() {
			return fn(ctx)
		}
	}
	if err := s.RequireTransactions(ctx); err != nil {
		return err
	}
	session, err := s.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) { return nil, fn(tx) })
	return err
}
