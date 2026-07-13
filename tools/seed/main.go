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
	"time"

	"github.com/google/uuid"
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

// tenantUUID derives the legacy UUID previously stored in billing/student/staff
// tenant_id columns (pre TEXT migration). Kept only to clean up old seed rows.
func tenantUUID(code string) string {
	return deterministicUUID("auraedu:tenant:" + code)
}

func deterministicUUID(seed string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed)).String()
}

func main() {
	ctx := context.Background()

	dbs := map[string]*pgxpool.Pool{}
	for name, dsn := range map[string]string{
		"identity": envOr("IDENTITY_DATABASE_URL", "postgres://auraedu:auraedu@localhost:5432/identity?sslmode=disable"),
		"tenant":   envOr("TENANT_DATABASE_URL", "postgres://auraedu:auraedu@localhost:5432/tenant?sslmode=disable"),
		"billing":  envOr("BILLING_DATABASE_URL", "postgres://auraedu:auraedu@localhost:5432/billing?sslmode=disable"),
		"student":  envOr("STUDENT_DATABASE_URL", "postgres://auraedu:auraedu@localhost:5432/student?sslmode=disable"),
		"staff":    envOr("STAFF_DATABASE_URL", "postgres://auraedu:auraedu@localhost:5432/staff?sslmode=disable"),
	} {
		pool, err := openPool(ctx, dsn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s db: %v\n", name, err)
			os.Exit(1)
		}
		defer pool.Close()
		dbs[name] = pool
	}

	if err := seedIdentity(ctx, dbs["identity"]); err != nil {
		fmt.Fprintf(os.Stderr, "seed identity: %v\n", err)
		os.Exit(1)
	}

	if err := seedTenants(ctx, dbs["tenant"]); err != nil {
		fmt.Fprintf(os.Stderr, "seed tenants: %v\n", err)
		os.Exit(1)
	}

	if err := seedBilling(ctx, dbs["billing"]); err != nil {
		fmt.Fprintf(os.Stderr, "seed billing: %v\n", err)
		os.Exit(1)
	}

	if err := seedStudents(ctx, dbs["student"]); err != nil {
		fmt.Fprintf(os.Stderr, "seed students: %v\n", err)
		os.Exit(1)
	}

	if err := seedStaff(ctx, dbs["staff"]); err != nil {
		fmt.Fprintf(os.Stderr, "seed staff: %v\n", err)
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

func seedBilling(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL app.is_platform_admin = true"); err != nil {
		return err
	}

	plans := []struct {
		id       string
		name     string
		code     string
		price    int
		interval string
		features []string
	}{
		{deterministicUUID("auraedu:plan:starter"), "Starter", "starter", 29900, "monthly", []string{"public_website", "student_management", "staff_management", "attendance", "report_cards"}},
		{deterministicUUID("auraedu:plan:growth"), "Growth", "growth", 59900, "monthly", []string{"public_website", "student_management", "staff_management", "attendance", "report_cards", "fees", "sms_notifications"}},
		{deterministicUUID("auraedu:plan:professional"), "Professional", "professional", 99900, "monthly", []string{"public_website", "student_management", "staff_management", "attendance", "report_cards", "fees", "sms_notifications", "analytics", "custom_domain"}},
	}

	planIDs := make(map[string]string)
	for _, p := range plans {
		features := "{}"
		if len(p.features) > 0 {
			quoted := make([]string, len(p.features))
			for i, f := range p.features {
				quoted[i] = "'" + strings.ReplaceAll(f, "'", "''") + "'"
			}
			features = "ARRAY[" + strings.Join(quoted, ",") + "]"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_plans (id, name, code, description, price_cents, currency, billing_interval, features, status, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, 'GHS', $6, `+features+`, 'active', now(), now())
			ON CONFLICT (code) DO UPDATE SET
			  name = EXCLUDED.name,
			  description = EXCLUDED.description,
			  price_cents = EXCLUDED.price_cents,
			  billing_interval = EXCLUDED.billing_interval,
			  features = EXCLUDED.features,
			  status = EXCLUDED.status,
			  updated_at = now()
		`, p.id, p.name, p.code, p.name+" plan", p.price, p.interval); err != nil {
			return fmt.Errorf("upsert plan %s: %w", p.code, err)
		}
		// The stored id may differ from p.id when the plan already existed
		// (ON CONFLICT keeps the original id), so read it back.
		var storedID string
		if err := tx.QueryRow(ctx, `SELECT id FROM billing_plans WHERE code = $1`, p.code).Scan(&storedID); err != nil {
			return fmt.Errorf("load plan id %s: %w", p.code, err)
		}
		planIDs[p.code] = storedID
	}

	now := time.Now().UTC()
	for _, t := range tenants {
		// Tenant IDs are tenant *codes* (e.g. upshs) — see billing migration 0003_tenant_id_text.
		subID := deterministicUUID("auraedu:subscription:" + t.Code)
		invoiceID := deterministicUUID("auraedu:invoice:" + t.Code)
		planID := planIDs[t.Plan]
		if planID == "" {
			planID = planIDs["starter"]
		}

		// Remove rows seeded by older versions of this tool, which stored a
		// derived UUID instead of the tenant code.
		legacyID := tenantUUID(t.Code)
		if _, err := tx.Exec(ctx, `DELETE FROM billing_invoices WHERE tenant_id = $1`, legacyID); err != nil {
			return fmt.Errorf("clean legacy invoices for %s: %w", t.Code, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM billing_subscriptions WHERE tenant_id = $1`, legacyID); err != nil {
			return fmt.Errorf("clean legacy subscriptions for %s: %w", t.Code, err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_subscriptions (id, tenant_id, plan_id, status, current_period_start, current_period_end, created_at, updated_at)
			VALUES ($1::uuid, $2, $3::uuid, 'active', $4, $5, now(), now())
			ON CONFLICT (id) DO UPDATE SET
			  plan_id = EXCLUDED.plan_id,
			  status = EXCLUDED.status,
			  current_period_start = EXCLUDED.current_period_start,
			  current_period_end = EXCLUDED.current_period_end,
			  updated_at = now()
		`, subID, t.Code, planID, now, now.AddDate(0, 1, 0)); err != nil {
			return fmt.Errorf("upsert subscription for %s: %w", t.Code, err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_invoices (id, tenant_id, subscription_id, amount_cents, status, due_date, paid_at, created_at, updated_at)
			VALUES ($1::uuid, $2, $3::uuid, $4, 'paid', $5, $6, now(), now())
			ON CONFLICT (id) DO UPDATE SET
			  amount_cents = EXCLUDED.amount_cents,
			  status = EXCLUDED.status,
			  due_date = EXCLUDED.due_date,
			  paid_at = EXCLUDED.paid_at,
			  updated_at = now()
		`, invoiceID, t.Code, subID, 29900, now.AddDate(0, 0, 14), now); err != nil {
			return fmt.Errorf("upsert invoice for %s: %w", t.Code, err)
		}
	}

	return tx.Commit(ctx)
}

func seedStudents(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL app.is_platform_admin = true"); err != nil {
		return err
	}

	students := []struct {
		code      string
		firstName string
		lastName  string
		gender    string
		dob       string
		tenant    string
	}{
		{"UPS-001", "Kwame", "Asante", "male", "2008-03-15", "upshs"},
		{"UPS-002", "Ama", "Owusu", "female", "2007-07-22", "upshs"},
		{"UPS-003", "Kofi", "Mensah", "male", "2008-11-05", "upshs"},
		{"ABM-001", "Yaa", "Darko", "female", "2007-05-10", "aboom"},
		{"ABM-002", "Ebenezer", "Agyemang", "male", "2008-01-30", "aboom"},
	}

	for _, s := range students {
		// Remove rows seeded by older versions of this tool, which stored a
		// derived UUID instead of the tenant code (see student migration 0004_tenant_id_text).
		if _, err := tx.Exec(ctx, `DELETE FROM students WHERE tenant_id = $1 AND student_code = $2`, tenantUUID(s.tenant), s.code); err != nil {
			return fmt.Errorf("clean legacy student %s: %w", s.code, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO students (id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 'active', now(), now())
			ON CONFLICT (tenant_id, student_code) DO UPDATE SET
			  first_name = EXCLUDED.first_name,
			  last_name = EXCLUDED.last_name,
			  date_of_birth = EXCLUDED.date_of_birth,
			  gender = EXCLUDED.gender,
			  status = EXCLUDED.status,
			  updated_at = now()
		`, s.tenant, s.firstName, s.lastName, s.code, s.dob, s.gender); err != nil {
			return fmt.Errorf("upsert student %s: %w", s.code, err)
		}
	}

	return tx.Commit(ctx)
}

func seedStaff(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL app.is_platform_admin = true"); err != nil {
		return err
	}

	staff := []struct {
		code      string
		firstName string
		lastName  string
		staffType string
		email     string
		tenant    string
	}{
		{"UPS-T01", "John", "Doe", "teacher", "john.doe@upshs.edu", "upshs"},
		{"UPS-N01", "Jane", "Smith", "non_teaching", "jane.smith@upshs.edu", "upshs"},
		{"ABM-T01", "Peter", "Brown", "teacher", "peter.brown@aboom.edu", "aboom"},
		{"ABM-N01", "Mary", "Johnson", "non_teaching", "mary.johnson@aboom.edu", "aboom"},
	}

	for _, s := range staff {
		// Remove rows seeded by older versions of this tool, which stored a
		// derived UUID instead of the tenant code (see staff migration 0003_tenant_id_text).
		if _, err := tx.Exec(ctx, `DELETE FROM staff WHERE tenant_id = $1 AND staff_code = $2`, tenantUUID(s.tenant), s.code); err != nil {
			return fmt.Errorf("clean legacy staff %s: %w", s.code, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO staff (id, tenant_id, first_name, last_name, staff_type, email, staff_code, status, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 'active', now(), now())
			ON CONFLICT (tenant_id, staff_code) DO UPDATE SET
			  first_name = EXCLUDED.first_name,
			  last_name = EXCLUDED.last_name,
			  staff_type = EXCLUDED.staff_type,
			  email = EXCLUDED.email,
			  status = EXCLUDED.status,
			  updated_at = now()
		`, s.tenant, s.firstName, s.lastName, s.staffType, s.email, s.code); err != nil {
			return fmt.Errorf("upsert staff %s: %w", s.code, err)
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
