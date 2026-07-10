// Package memory is a seeded in-memory user store. It lets the Identity Service run
// and be tested without infrastructure; the Postgres adapter (argon2id, per-tenant
// users) is a later story. Demo passwords are all "password123" — never for prod.
package memory

import (
	"strings"

	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/ports"
)

type entry struct {
	user domain.User
	cred domain.Credential
}

type Repository struct {
	byEmail map[string]entry
}

var _ ports.Repository = (*Repository)(nil)

const demoPassword = "password123"

// New seeds a few users across roles/tenants.
func New() (*Repository, error) {
	r := &Repository{byEmail: make(map[string]entry)}
	seed := []domain.User{
		{ID: "u-teacher", Email: "e.mensah@upshs.edu.gh", Name: "Efua Mensah", TenantID: "upshs", Role: "teacher",
			Permissions: []string{"attendance.mark", "assessments.record_scores", "reports.read"}},
		{ID: "u-admin", Email: "admin@upshs.edu.gh", Name: "School Admin", TenantID: "upshs", Role: "school_admin",
			Permissions: []string{"features.manage", "students.create", "students.update", "staff.create"}},
		{ID: "u-super", Email: "super@auraedu.dev", Name: "Platform Admin", TenantID: "", Role: "platform_super_admin",
			Permissions: nil},
	}
	for _, u := range seed {
		cred, err := domain.NewCredential(demoPassword)
		if err != nil {
			return nil, err
		}
		r.byEmail[strings.ToLower(u.Email)] = entry{user: u, cred: cred}
	}
	return r, nil
}

func (r *Repository) FindByEmail(email string) (domain.User, domain.Credential, bool) {
	e, ok := r.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return domain.User{}, domain.Credential{}, false
	}
	return e.user, e.cred, true
}
