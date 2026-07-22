package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/auraedu/identity-service/internal/domain"
)

func main() {
	password := "Password123"
	if len(os.Args) > 1 {
		password = os.Args[1]
	}

	cred, err := domain.NewCredential(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to hash password: %v\n", err)
		os.Exit(1)
	}

	params, err := json.Marshal(cred.Params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal params: %v\n", err)
		os.Exit(1)
	}

	perms := []string{
		"features.manage", "users.read", "users.create", "users.update", "roles.assign",
		"students.read", "students.create", "students.update", "students.delete",
		"staff.read", "staff.create", "staff.update",
		"academic.read", "academic.manage",
		"attendance.read", "attendance.mark",
		"assessments.read", "assessments.record_scores", "assessments.manage",
		"reports.read", "reports.publish",
		"fees.read", "fees.manage",
		"payments.read", "payments.initiate", "payments.configure",
		"notifications.read", "notifications.send", "notifications.manage",
		"website.read", "website.manage",
		"files.read", "files.upload", "files.delete",
		"analytics.view",
		"billing.read", "billing.manage",
		"cbt.read", "cbt.author", "cbt.take", "cbt.grade",
		"ai.view_recommendations", "ai.approve_recommendations", "ai.view_predictions", "ai.approve_predictions", "ai.view_guidance", "ai.approve_guidance",
		"audit.read",
	}
	var permsArray strings.Builder
	permsArray.WriteString("ARRAY[")
	for i, p := range perms {
		if i > 0 {
			permsArray.WriteString(",")
		}
		permsArray.WriteString("'")
		permsArray.WriteString(p)
		permsArray.WriteString("'")
	}
	permsArray.WriteString("]")

	fmt.Printf(`-- Seed platform super-admin (password: %s)
INSERT INTO users (id, tenant_id, email, name, role, permissions, status, created_at, updated_at)
VALUES (
  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
  NULL,
  'superadmin@auraedu.com',
  'Super Admin',
  'platform_super_admin',
  %s,
  'active',
  now(),
  now()
)
ON CONFLICT (tenant_id, email) DO UPDATE SET
  name = EXCLUDED.name,
  role = EXCLUDED.role,
  permissions = EXCLUDED.permissions,
  status = EXCLUDED.status,
  updated_at = now();

INSERT INTO credentials (user_id, tenant_id, algo, salt, hash, params, updated_at)
VALUES (
  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
  NULL,
  'argon2id',
  '\x%x'::bytea,
  '\x%x'::bytea,
  '%s'::jsonb,
  now()
)
ON CONFLICT (user_id) DO UPDATE SET
  algo = EXCLUDED.algo,
  salt = EXCLUDED.salt,
  hash = EXCLUDED.hash,
  params = EXCLUDED.params,
  updated_at = now();
`, password, permsArray.String(), cred.Salt, cred.Hash, string(params))
}
