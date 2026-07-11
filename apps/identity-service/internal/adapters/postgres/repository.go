// Package postgres is the runtime Repository implementation for Identity Service.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ ports.Repository = (*Repository)(nil)

func (r *Repository) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return r.withActorTx(ctx, tenancy.ActorFromContext(ctx), fn)
}

func (r *Repository) withPrivilegedTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return r.withActorTx(ctx, auth.Actor{PlatformAdmin: true}, fn)
}

func (r *Repository) withActorTx(ctx context.Context, actor auth.Actor, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := db.SetTenantContext(ctx, tx, actor.TenantID, actor.PlatformAdmin); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (domain.User, domain.Credential, bool, error) {
	var u domain.User
	var cred domain.Credential
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT u.id, u.email, u.name, u.tenant_id, u.role, u.permissions, u.status,
			       c.algo, c.salt, c.hash, c.params
			FROM users u
			LEFT JOIN credentials c ON c.user_id = u.id
			WHERE u.email = LOWER(TRIM($1))
		`, email)
		var tenantID *string
		var paramsJSON []byte
		err := row.Scan(&u.ID, &u.Email, &u.Name, &tenantID, &u.Role, &u.Permissions, &u.Status,
			&cred.Algo, &cred.Salt, &cred.Hash, &paramsJSON)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if tenantID != nil {
			u.TenantID = *tenantID
		}
		return jsonUnmarshal(paramsJSON, &cred.Params)
	})
	if err != nil {
		return domain.User{}, domain.Credential{}, false, err
	}
	if u.ID == "" {
		return domain.User{}, domain.Credential{}, false, nil
	}
	return u, cred, true, nil
}

func (r *Repository) CreateUser(ctx context.Context, u domain.User, cred domain.Credential) (string, error) {
	var id string
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO users (tenant_id, email, name, role, permissions, status)
			VALUES ($1, LOWER(TRIM($2)), $3, $4, $5, $6)
			RETURNING id
		`, strPtr(u.TenantID), u.Email, u.Name, u.Role, u.Permissions, u.Status)
		if err := row.Scan(&id); err != nil {
			return err
		}
		if cred.Hash != nil {
			paramsJSON, err := jsonMarshal(cred.Params)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO credentials (user_id, tenant_id, algo, salt, hash, params)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, id, strPtr(u.TenantID), cred.Algo, cred.Salt, cred.Hash, paramsJSON)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

func (r *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	var out []domain.User
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, email, name, tenant_id, role, permissions, status FROM users ORDER BY created_at DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			u, err := scanUser(rows)
			if err != nil {
				return err
			}
			out = append(out, u)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) GetUser(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `SELECT id, email, name, tenant_id, role, permissions, status FROM users WHERE id = $1`, id)
		var err error
		u, err = scanUser(row)
		return err
	})
	return u, err
}

func (r *Repository) UpdateUser(ctx context.Context, id string, u domain.User) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		type set struct {
			col string
			val any
		}
		var sets []set
		if u.Name != "" {
			sets = append(sets, set{col: "name", val: u.Name})
		}
		if u.Role != "" {
			sets = append(sets, set{col: "role", val: u.Role})
		}
		if u.Permissions != nil {
			sets = append(sets, set{col: "permissions", val: u.Permissions})
		}
		if u.Status != "" {
			sets = append(sets, set{col: "status", val: u.Status})
		}
		if len(sets) == 0 {
			return nil
		}
		args := []any{id}
		parts := make([]string, 0, len(sets)+1)
		for i, s := range sets {
			parts = append(parts, fmt.Sprintf("%s = $%d", s.col, i+2))
			args = append(args, s.val)
		}
		parts = append(parts, "updated_at = NOW()")
		sql := "UPDATE users SET " + join(parts, ", ") + " WHERE id = $1"
		_, err := tx.Exec(ctx, sql, args...)
		return err
	})
}

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
		return err
	})
}

func (r *Repository) UpdateCredential(ctx context.Context, userID string, cred domain.Credential) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		paramsJSON, err := jsonMarshal(cred.Params)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO credentials (user_id, tenant_id, algo, salt, hash, params)
			VALUES ($1, (SELECT tenant_id FROM users WHERE id = $1), $2, $3, $4, $5)
			ON CONFLICT (user_id) DO UPDATE SET
				algo = EXCLUDED.algo,
				salt = EXCLUDED.salt,
				hash = EXCLUDED.hash,
				params = EXCLUDED.params,
				updated_at = NOW()
		`, userID, cred.Algo, cred.Salt, cred.Hash, paramsJSON)
		return err
	})
}

func (r *Repository) SaveRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO refresh_tokens (user_id, tenant_id, token_hash, expires_at)
			VALUES ($1, (SELECT tenant_id FROM users WHERE id = $1), $2, $3)
		`, userID, tokenHash, expiresAt)
		return err
	})
}

func (r *Repository) FindRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT user_id FROM refresh_tokens
			WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
		`, tokenHash)
		return row.Scan(&userID)
	})
	return userID, err
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1`, tokenHash)
		return err
	})
}

func (r *Repository) SavePasswordResetToken(ctx context.Context, tenantID, userID, tokenHash string, expiresAt time.Time) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO password_resets (tenant_id, user_id, token_hash, expires_at)
			VALUES ($1, $2, $3, $4)
		`, strPtr(tenantID), userID, tokenHash, expiresAt)
		return err
	})
}

func (r *Repository) UsePasswordResetToken(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE password_resets
			SET used_at = NOW()
			WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
			RETURNING user_id
		`, tokenHash)
		return row.Scan(&userID)
	})
	return userID, err
}

func (r *Repository) SaveInvite(
	ctx context.Context,
	tenantID, email, role string,
	permissions []string,
	tokenHash string,
	invitedBy *string,
	expiresAt time.Time,
) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO invites (tenant_id, email, role, permissions, token_hash, invited_by, expires_at)
			VALUES ($1, LOWER(TRIM($2)), $3, $4, $5, $6, $7)
		`, strPtr(tenantID), email, role, permissions, tokenHash, invitedBy, expiresAt)
		return err
	})
}

func (r *Repository) UseInvite(ctx context.Context, tokenHash string) (ports.InviteDetails, error) {
	var d ports.InviteDetails
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE invites
			SET used_at = NOW()
			WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
			RETURNING tenant_id, email, role, permissions
		`, tokenHash)
		var tenantID *string
		if err := row.Scan(&tenantID, &d.Email, &d.Role, &d.Permissions); err != nil {
			return err
		}
		if tenantID != nil {
			d.TenantID = *tenantID
		}
		return nil
	})
	return d, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(s scanner) (domain.User, error) {
	var u domain.User
	var tenantID *string
	err := s.Scan(&u.ID, &u.Email, &u.Name, &tenantID, &u.Role, &u.Permissions, &u.Status)
	if tenantID != nil {
		u.TenantID = *tenantID
	}
	return u, err
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}
