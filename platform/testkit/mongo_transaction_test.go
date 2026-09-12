package testkit_test

import (
	"context"
	"errors"
	"testing"

	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/testkit"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoReplicaSetTransactionRollback(t *testing.T) {
	ctx := context.Background()
	db := testkit.NewMongoReplicaSet(ctx, t)
	scope, err := db.Store.Collection("rollback").ScopeTo("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	other, err := pmongo.Open(ctx, pmongo.Config{URI: db.URI, Database: "foreign"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := other.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	if err := db.Store.WithTransaction(ctx, func(tx context.Context) error {
		called := false
		err := other.WithTransaction(tx, func(context.Context) error { called = true; return nil })
		if err == nil || called {
			t.Fatal("foreign client session accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("abort nested work")
	err = db.Store.WithTransaction(ctx, func(tx context.Context) error {
		if _, err := scope.InsertOne(tx, bson.M{"_id": "outer"}); err != nil {
			return err
		}
		return db.Store.WithTransaction(tx, func(nested context.Context) error {
			if _, err := scope.InsertOne(nested, bson.M{"_id": "inner"}); err != nil {
				return err
			}
			return sentinel
		})
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected callback error, got %v", err)
	}
	count, err := scope.CountDocuments(ctx, bson.M{})
	if err != nil || count != 0 {
		t.Fatalf("rollback count=%d err=%v", count, err)
	}
	if err := db.Store.WithTransaction(ctx, func(tx context.Context) error { _, err := scope.InsertOne(tx, bson.M{"_id": "committed"}); return err }); err != nil {
		t.Fatal(err)
	}
	count, err = scope.CountDocuments(ctx, bson.M{})
	if err != nil || count != 1 {
		t.Fatalf("commit count=%d err=%v", count, err)
	}
}

func TestMongoStandaloneRejectsTransactionBeforeCallback(t *testing.T) {
	ctx := context.Background()
	db := testkit.NewMongo(ctx, t)
	called := false
	err := db.Store.WithTransaction(ctx, func(context.Context) error { called = true; return nil })
	if err == nil || called {
		t.Fatalf("standalone accepted: called=%t err=%v", called, err)
	}
}
