// Command seed inserts baseline demo data for local AuraEDU development.
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

const (
	argon2Time    = 1
	argon2Memory  = 16 * 1024
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

type argonParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
}

type credential struct {
	Salt   []byte
	Hash   []byte
	Algo   string
	Params argonParams
}

func hashPassword(password string) (credential, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return credential{}, err
	}
	params := argonParams{Time: argon2Time, Memory: argon2Memory, Threads: argon2Threads, KeyLen: argon2KeyLen}
	hash := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	return credential{Salt: salt, Hash: hash, Algo: "argon2id", Params: params}, nil
}

func verifyPassword(password string, c credential) bool {
	got := argon2.IDKey([]byte(password), c.Salt, c.Params.Time, c.Params.Memory, c.Params.Threads, c.Params.KeyLen)
	return subtle.ConstantTimeCompare(got, c.Hash) == 1
}

type user struct {
	ID          string
	TenantID    *string
	Email       string
	Name        string
	Role        string
	Permissions []string
	Password    string
}

type tenant struct {
	Code   string
	Name   string
	Short  string
	Domain string
	Plan   string
	Status string
}

var defaultPassword = envOr("SEED_PASSWORD", "Password123")

var platformSuperAdmin = user{
	ID:       "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
	Email:    "superadmin@auraedu.com",
	Name:     "Super Admin",
	Role:     "platform_super_admin",
	Password: defaultPassword,
	Permissions: []string{
		"features.manage", "users.read", "users.create", "users.update", "roles.assign",
		"students.read", "students.create", "students.update", "students.delete",
		"staff.read", "staff.create", "staff.update",
		"academic.read", "academic.manage",
		"attendance.read", "attendance.mark",
		"assessments.read", "assessments.record_scores", "assessments.manage",
		"reports.read", "reports.publish",
		"fees.read", "fees.manage",
		"payments.read", "payments.initiate",
		"notifications.read", "notifications.send", "notifications.manage",
		"website.read", "website.manage",
		"files.read", "files.upload", "files.delete",
		"analytics.view",
		"billing.read", "billing.manage",
		"cbt.read", "cbt.author", "cbt.take", "cbt.grade",
		"audit.read",
	},
}

var tenants = []tenant{
	{Code: "upshs", Name: "Union Preparatory SHS", Short: "UPSHS", Domain: "upshs.auraedu.com", Plan: "starter", Status: "active"},
	{Code: "aboom", Name: "Aboom Senior High", Short: "Aboom", Domain: "aboom.auraedu.com", Plan: "starter", Status: "active"},
}

var tenantAdmins = []user{
	{ID: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12", TenantID: strPtr("upshs"), Email: "admin@upshs.edu", Name: "UPSHS Admin", Role: "admin", Password: defaultPassword},
	{ID: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13", TenantID: strPtr("aboom"), Email: "admin@aboom.edu", Name: "Aboom Admin", Role: "admin", Password: defaultPassword},
}

var defaultFeatures = []string{
	"public_website",
	"student_management",
	"staff_management",
	"parent_portal",
	"teacher_portal",
	"attendance",
	"report_cards",
	"announcements",
	"email_notifications",
	"billing",
	"file_management",
}

func main() {
	ctx := context.Background()

	identityDB, err := openPool(ctx, envOr("IDENTITY_DATABASE_URL", "postgres://auraedu:auraedu@localhost:5432/identity?sslmode=disable"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "identity db: %v\n", err)
		os.Exit(1)
	}
	defer identityDB.Close()

	tenantDB, err := openPool(ctx, envOr("TENANT_DATABASE_URL", "postgres://auraedu:auraedu@localhost:5432/tenant?sslmode=disable"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "tenant db: %v\n", err)
		os.Exit(1)
	}
	defer tenantDB.Close()

	if err := seedIdentity(ctx, identityDB); err != nil {
		fmt.Fprintf(os.Stderr, "seed identity: %v\n", err)
		os.Exit(1)
	}

	if err := seedTenants(ctx, tenantDB); err != nil {
		fmt.Fprintf(os.Stderr, "seed tenants: %v\n", err)
		os.Exit(1)
	}

	if err := writeCredentials(filepath.Join(repoRoot(), "credentials.txt")); err != nil {
		fmt.Fprintf(os.Stderr, "write credentials: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Seed complete. See credentials.txt for login details.")
}

func seedIdentity(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL app.is_platform_admin = true"); err != nil {
		return err
	}

	superCred, err := hashPassword(platformSuperAdmin.Password)
	if err != nil {
		return err
	}

	allUsers := append([]user{platformSuperAdmin}, tenantAdmins...)
	for _, u := range allUsers {
		cred, err := hashPassword(u.Password)
		if err != nil {
			return err
		}
		perms := "'{}'::text[]"
		if len(u.Permissions) > 0 {
			quoted := make([]string, len(u.Permissions))
			for i, p := range u.Permissions {
				quoted[i] = "'" + strings.ReplaceAll(p, "'", "''") + "'"
			}
			perms = "ARRAY[" + strings.Join(quoted, ",") + "]"
		}
		var tenantID interface{}
		if u.TenantID != nil {
			tenantID = *u.TenantID
		} else {
			tenantID = nil
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, tenant_id, email, name, role, permissions, status, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, `+perms+`, 'active', now(), now())
			ON CONFLICT (id) DO UPDATE SET
			  name = EXCLUDED.name,
			  role = EXCLUDED.role,
			  permissions = EXCLUDED.permissions,
			  status = EXCLUDED.status,
			  updated_at = now()
		`, u.ID, tenantID, u.Email, u.Name, u.Role); err != nil {
			return fmt.Errorf("upsert user %s: %w", u.Email, err)
		}

		paramsJSON, err := json.Marshal(cred.Params)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO credentials (user_id, tenant_id, algo, salt, hash, params, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6::jsonb, now())
			ON CONFLICT (user_id) DO UPDATE SET
			  tenant_id = EXCLUDED.tenant_id,
			  algo = EXCLUDED.algo,
			  salt = EXCLUDED.salt,
			  hash = EXCLUDED.hash,
			  params = EXCLUDED.params,
			  updated_at = now()
		`, u.ID, tenantID, cred.Algo, cred.Salt, cred.Hash, paramsJSON); err != nil {
			return fmt.Errorf("upsert credentials %s: %w", u.Email, err)
		}

		_ = superCred
	}

	return tx.Commit(ctx)
}

func seedTenants(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, t := range tenants {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.tenant_id = '%s'", strings.ReplaceAll(t.Code, "'", "''"))); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO tenants (code, name, short, status, domain, plan, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, now(), now())
			ON CONFLICT (code) DO UPDATE SET
			  name = EXCLUDED.name,
			  short = EXCLUDED.short,
			  status = EXCLUDED.status,
			  domain = EXCLUDED.domain,
			  plan = EXCLUDED.plan,
			  updated_at = now()
		`, t.Code, t.Name, t.Short, t.Status, t.Domain, t.Plan); err != nil {
			return fmt.Errorf("upsert tenant %s: %w", t.Code, err)
		}

		for _, f := range defaultFeatures {
			if _, err := tx.Exec(ctx, `
				INSERT INTO tenant_features (tenant_code, feature_key, is_enabled, updated_at)
				VALUES ($1, $2, true, now())
				ON CONFLICT (tenant_code, feature_key) DO UPDATE SET
				  is_enabled = EXCLUDED.is_enabled,
				  updated_at = now()
			`, t.Code, f); err != nil {
				return fmt.Errorf("upsert feature %s for %s: %w", f, t.Code, err)
			}
		}
	}

	return tx.Commit(ctx)
}

func writeCredentials(path string) error {
	var b strings.Builder
	b.WriteString("AuraEDU local seed credentials\n")
	b.WriteString("================================\n\n")
	b.WriteString(fmt.Sprintf("%-30s %-30s %s\n", "Email", "Role", "Password"))
	b.WriteString(fmt.Sprintf("%-30s %-30s %s\n", platformSuperAdmin.Email, platformSuperAdmin.Role, platformSuperAdmin.Password))
	for _, u := range tenantAdmins {
		b.WriteString(fmt.Sprintf("%-30s %-30s %s\n", u.Email, u.Role+" ("+*u.TenantID+")", u.Password))
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func openPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

func repoRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func strPtr(s string) *string {
	return &s
}
