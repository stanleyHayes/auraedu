package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	mongoadapter "github.com/auraedu/identity-service/internal/adapters/mongo"
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/store"
	"github.com/spf13/cobra"
)

func seedDemoCommand() *cobra.Command {
	return &cobra.Command{Use: "seed-demo", Short: "Create an explicit development-only Mongo demo login", RunE: func(_ *cobra.Command, _ []string) error {
		if config.Getenv("ENVIRONMENT", "development") != "development" {
			return errors.New("seed-demo is available only in development")
		}
		selected, err := store.Selected()
		if err != nil {
			return err
		}
		if !selected.IsMongo() {
			return errors.New("seed-demo requires DATABASE_DRIVER=mongodb")
		}
		tenant, email, password := os.Getenv("DEMO_TENANT_ID"), os.Getenv("DEMO_USER_EMAIL"), os.Getenv("DEMO_USER_PASSWORD")
		if tenant == "" || email == "" || len(password) < 12 {
			return errors.New("DEMO_TENANT_ID, DEMO_USER_EMAIL and DEMO_USER_PASSWORD (at least 12 characters) are required")
		}
		role := config.Getenv("DEMO_USER_ROLE", "teacher")
		if role != "teacher" && role != "school_admin" {
			return errors.New("DEMO_USER_ROLE must be teacher or school_admin")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repo, database, err := mongoadapter.OpenFromEnv(ctx)
		if err != nil {
			return err
		}
		defer func() {
			if err := database.Close(context.Background()); err != nil {
				slog.Error("close demo identity store", "err", err)
			}
		}()
		cred, err := domain.NewCredential(password)
		if err != nil {
			return err
		}
		permissions := []string{"students.read",
			"academic.read",
			"attendance.read",
			"attendance.mark",
			"assessments.read",
			"assessments.record_scores",
			"reports.read",
			"notifications.read",
			"fees.read",
			"payments.read",
			"billing.read",
			"cbt.read",
			"cbt.take"}
		if role == "school_admin" {
			permissions = append(permissions,
				"features.manage",
				"users.read",
				"users.create",
				"users.update",
				"roles.assign",
				"students.create",
				"students.update",
				"staff.create",
				"fees.manage",
				"payments.initiate",
				"payments.configure",
				"billing.manage",
				"reports.publish",
				"cbt.author",
				"cbt.grade",
				"notifications.send",
				"notifications.manage",
				"admissions.application.read",
				"admissions.application.review")
		}
		ctx = tenancy.WithActor(ctx, auth.Actor{PlatformAdmin: true})
		_,
			err = repo.CreateUser(ctx,
			domain.User{TenantID: tenant,
				Email: strings.TrimSpace(email),
				Name: config.Getenv("DEMO_USER_NAME",
					"Demo User"),
				Role:        role,
				Permissions: permissions,
				Status:      domain.StatusActive},
			cred)
		if errors.Is(err, domain.ErrConflict) {
			return fmt.Errorf("demo identity already exists; seed-demo never replaces credentials")
		}
		return err
	}}
}
