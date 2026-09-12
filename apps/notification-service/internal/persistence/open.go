// Package persistence selects complete Postgres or MongoDB adapters at startup.
package persistence

import (
	"context"
	"log/slog"

	adapter "github.com/auraedu/notification-service/internal/adapters/mongo"
	"github.com/auraedu/notification-service/internal/adapters/postgres"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/store"
)

type Messages interface {
	ports.MessageRepository
	ports.ScheduledMessageRepository
	ports.DurableDeliveryRepository
	ports.DeliveryFeedbackRepository
	ports.OutboxRepository
}
type Connection struct {
	Messages      Messages
	Templates     ports.TemplateRepository
	Subscriptions ports.SubscriptionRepository
	Announcements ports.AnnouncementRepository
	Devices       ports.DeviceTokenRepository
	Journeys      ports.JourneyRepository
	Processed     ports.ProcessedEventRepository
	Driver        string
	Close         func()
	Ping          func(context.Context) error
}

func Open(ctx context.Context) (*Connection, error) {
	driver, err := store.Selected()
	if err != nil {
		return nil, err
	}
	if driver.IsMongo() {
		s, err := pmongo.Open(ctx, pmongo.Config{URI: config.Getenv("MONGODB_URI",
			""), Database: config.Getenv("MONGODB_DATABASE", "notification-service"),
			MaxPoolSize: 2})
		if err != nil {
			return nil, err
		}
		if err = s.RequireTransactions(ctx); err != nil {
			closeMongo(ctx, s)
			return nil, err
		}

		if err = adapter.EnsureIndexes(ctx, s); err != nil {
			closeMongo(ctx, s)
			return nil, err
		}
		return &Connection{adapter.NewMessageRepository(s), adapter.NewTemplateRepository(s),
				adapter.NewSubscriptionRepository(s), adapter.NewAnnouncementRepository(s),
				adapter.NewDeviceTokenRepository(s), adapter.NewJourneyRepository(s),
				adapter.NewProcessedEventRepository(s), string(driver),
				func() { closeMongo(context.Background(), s) }, s.Ping},
			nil
	}
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	s, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: config.Getenv("MIGRATIONS_PATH", "migrations")})
	if err != nil {
		return nil, err
	}
	return &Connection{postgres.NewMessageRepository(s), postgres.NewTemplateRepository(s),
		postgres.NewSubscriptionRepository(s), postgres.NewAnnouncementRepository(s),
		postgres.NewDeviceTokenRepository(s), postgres.NewJourneyRepository(s),
		postgres.NewProcessedEventRepository(s), string(driver),
		s.Close, s.Ping}, nil
}

func closeMongo(ctx context.Context, s *pmongo.Store) {
	if err := s.Close(ctx); err != nil {
		slog.Warn("close MongoDB connection", "error", err)
	}
}
