// Package postgres is the runtime Repository implementation for Identity Service.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
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
	tenantID := tenancy.ActorFromContext(ctx).TenantID
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT u.id, u.email, u.name, u.tenant_id, u.role, u.permissions, u.status,
			       c.algo, c.salt, c.hash, c.params
			FROM users u
			LEFT JOIN credentials c ON c.user_id = u.id
			WHERE u.email = LOWER(TRIM($1))
			  AND (u.tenant_id = NULLIF($2, '') OR u.tenant_id IS NULL)
			ORDER BY CASE WHEN u.tenant_id = NULLIF($2, '') THEN 0 ELSE 1 END
			LIMIT 1
		`, email, tenantID)
		var tenantID, algorithm *string
		var paramsJSON []byte
		err := row.Scan(&u.ID, &u.Email, &u.Name, &tenantID, &u.Role, &u.Permissions, &u.Status,
			&algorithm, &cred.Salt, &cred.Hash, &paramsJSON)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if tenantID != nil {
			u.TenantID = *tenantID
		}
		if algorithm == nil {
			return nil
		}
		cred.Algo = *algorithm
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

func (r *Repository) InspectInvite(ctx context.Context, tokenHash string) (ports.InviteDetails, error) {
	var details ports.InviteDetails
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT tenant_id, email, role, permissions
			FROM invites
			WHERE token_hash = $1
			  AND revoked_at IS NULL
			  AND expires_at > NOW()
		`, tokenHash).Scan(&details.TenantID, &details.Email, &details.Role, &details.Permissions)
	})
	if err != nil {
		return ports.InviteDetails{}, err
	}
	return details, nil
}

func (r *Repository) AcceptInviteWithCredential(
	ctx context.Context,
	tokenHash, name, password string,
	cred domain.Credential,
) (domain.User, error) {
	var accepted domain.User
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		var inviteID string
		var details ports.InviteDetails
		if err := tx.QueryRow(ctx, `
			SELECT id, tenant_id, email, role, permissions
			FROM invites
			WHERE token_hash = $1
			  AND revoked_at IS NULL
			  AND expires_at > NOW()
			FOR UPDATE
		`, tokenHash).Scan(&inviteID, &details.TenantID, &details.Email, &details.Role, &details.Permissions); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrExpiredToken
			}
			return err
		}

		var acceptErr error
		accepted, acceptErr = acceptInviteUser(ctx, tx, details, name, password, cred)
		if acceptErr != nil {
			return acceptErr
		}

		_, updateErr := tx.Exec(ctx, `UPDATE invites SET used_at = COALESCE(used_at, NOW()) WHERE id = $1`, inviteID)
		return updateErr
	})
	if err != nil {
		return domain.User{}, err
	}
	return accepted, nil
}

func acceptInviteUser(
	ctx context.Context,
	tx pgx.Tx,
	details ports.InviteDetails,
	name, password string,
	credential domain.Credential,
) (domain.User, error) {
	var accepted domain.User
	err := tx.QueryRow(ctx, `
		SELECT id, email, name, tenant_id, role, permissions, status
		FROM users
		WHERE tenant_id = $1 AND email = LOWER(TRIM($2))
		FOR UPDATE
	`, details.TenantID, details.Email).Scan(
		&accepted.ID, &accepted.Email, &accepted.Name, &accepted.TenantID,
		&accepted.Role, &accepted.Permissions, &accepted.Status,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		accepted = domain.User{
			Email: strings.ToLower(strings.TrimSpace(details.Email)), Name: strings.TrimSpace(name),
			TenantID: details.TenantID, Role: details.Role, Permissions: append([]string{}, details.Permissions...),
			Status: domain.StatusActive,
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (tenant_id, email, name, role, permissions, status)
			VALUES ($1, LOWER(TRIM($2)), $3, $4, $5, $6)
			RETURNING id
		`, accepted.TenantID, accepted.Email, accepted.Name, accepted.Role, accepted.Permissions, accepted.Status).Scan(&accepted.ID); err != nil {
			return domain.User{}, err
		}
		return accepted, insertCredential(ctx, tx, accepted.ID, accepted.TenantID, credential)
	case err != nil:
		return domain.User{}, err
	default:
		return accepted, validateInviteCredential(ctx, tx, accepted, details, password, credential)
	}
}

func validateInviteCredential(
	ctx context.Context,
	tx pgx.Tx,
	accepted domain.User,
	details ports.InviteDetails,
	password string,
	credential domain.Credential,
) error {
	if accepted.Role != details.Role || accepted.Status != domain.StatusActive ||
		!slices.Equal(accepted.Permissions, details.Permissions) {
		return domain.ErrExpiredToken
	}
	var existing domain.Credential
	var paramsJSON []byte
	err := tx.QueryRow(ctx, `SELECT algo, salt, hash, params FROM credentials WHERE user_id = $1 FOR UPDATE`, accepted.ID).
		Scan(&existing.Algo, &existing.Salt, &existing.Hash, &paramsJSON)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return insertCredential(ctx, tx, accepted.ID, accepted.TenantID, credential)
	case err != nil:
		return err
	default:
		if err := jsonUnmarshal(paramsJSON, &existing.Params); err != nil {
			return err
		}
		if !existing.Verify(password) {
			return domain.ErrExpiredToken
		}
		return nil
	}
}

func insertCredential(ctx context.Context, tx pgx.Tx, userID, tenantID string, cred domain.Credential) error {
	paramsJSON, err := jsonMarshal(cred.Params)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO credentials (user_id, tenant_id, algo, salt, hash, params)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, tenantID, cred.Algo, cred.Salt, cred.Hash, paramsJSON)
	return err
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
		return updateUser(ctx, tx, id, u)
	})
}

func (r *Repository) UpdateUserWithRoleChange(ctx context.Context, id string, u domain.User, event ports.RoleChangeEvent) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := updateUser(ctx, tx, id, u); err != nil {
			return err
		}
		if err := revokeUserSessions(ctx, tx, id); err != nil {
			return err
		}
		payload := ports.RoleChangeEventData(event)
		encoded, err := jsonMarshal(payload)
		if err != nil {
			return fmt.Errorf("identity outbox: encode role change: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO identity_outbox (tenant_id, event_type, payload)
			VALUES ($1, 'user.role_changed.v1', $2)
		`, event.TenantID, encoded); err != nil {
			return fmt.Errorf("identity outbox: enqueue role change: %w", err)
		}
		return nil
	})
}

func (r *Repository) UpdateUserAndRevokeSessions(ctx context.Context, id string, u domain.User) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := updateUser(ctx, tx, id, u); err != nil {
			return err
		}
		return revokeUserSessions(ctx, tx, id)
	})
}

func revokeUserSessions(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("identity authorization: revoke sessions: %w", err)
	}
	return nil
}

func updateUser(ctx context.Context, tx pgx.Tx, id string, u domain.User) error {
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
	for i, item := range sets {
		parts = append(parts, fmt.Sprintf("%s = $%d", item.col, i+2))
		args = append(args, item.val)
	}
	parts = append(parts, "updated_at = NOW()")
	sql := "UPDATE users SET " + join(parts, ", ") + " WHERE id = $1"
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
		return err
	})
}

func (r *Repository) ResetPasswordWithToken(ctx context.Context, tokenHash, tenantID string, cred domain.Credential) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		var userID string
		if err := tx.QueryRow(ctx, `
			UPDATE password_resets
			SET used_at = NOW()
			WHERE token_hash = $1
			  AND tenant_id = NULLIF($2, '')
			  AND used_at IS NULL AND revoked_at IS NULL
			  AND expires_at > NOW()
			RETURNING user_id
		`, tokenHash, tenantID).Scan(&userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrExpiredToken
			}
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE password_resets
			SET revoked_at = NOW()
			WHERE user_id = $1 AND token_hash <> $2
			  AND used_at IS NULL AND revoked_at IS NULL
		`, userID, tokenHash); err != nil {
			return err
		}
		return resetCredentialAndSessions(ctx, tx, userID, cred)
	})
}

func resetCredentialAndSessions(ctx context.Context, tx pgx.Tx, userID string, cred domain.Credential) error {
	paramsJSON, err := jsonMarshal(cred.Params)
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
			INSERT INTO credentials (user_id, tenant_id, algo, salt, hash, params)
			VALUES ($1, (SELECT tenant_id FROM users WHERE id = $1), $2, $3, $4, $5)
			ON CONFLICT (user_id) DO UPDATE SET
				algo = EXCLUDED.algo,
				salt = EXCLUDED.salt,
				hash = EXCLUDED.hash,
				params = EXCLUDED.params,
				updated_at = NOW()
	`, userID, cred.Algo, cred.Salt, cred.Hash, paramsJSON)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	_, err = tx.Exec(ctx, `
			UPDATE refresh_tokens
			SET revoked_at = COALESCE(revoked_at, NOW())
			WHERE user_id = $1
	`, userID)
	return err
}

func (r *Repository) GetMFA(ctx context.Context, userID string) (ports.MFARecord, bool, error) {
	var record ports.MFARecord
	found := false
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT secret_cipher, last_counter
			FROM user_mfa
			WHERE user_id = $1
		`, userID).Scan(&record.EncryptedSecret, &record.LastCounter)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err == nil {
			found = true
		}
		return err
	})
	return record, found, err
}

func (r *Repository) SaveMFA(ctx context.Context, userID string, encryptedSecret []byte, acceptedCounter int64) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			INSERT INTO user_mfa (user_id, tenant_id, secret_cipher, last_counter)
			SELECT id, tenant_id, $2, $3 FROM users WHERE id = $1
		`, userID, encryptedSecret, acceptedCounter)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		_, err = tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, NOW()) WHERE user_id = $1`, userID)
		return err
	})
}

func (r *Repository) AdvanceMFACounter(ctx context.Context, userID string, acceptedCounter int64) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE user_mfa
			SET last_counter = $2, updated_at = NOW()
			WHERE user_id = $1 AND last_counter < $2
		`, userID, acceptedCounter)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return domain.ErrInvalidCredentials
		}
		return nil
	})
}

func (r *Repository) SaveRefreshToken(ctx context.Context, userID, tokenHash, familyID string, expiresAt time.Time) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO refresh_tokens (user_id, tenant_id, token_hash, family_id, expires_at)
			VALUES ($1, (SELECT tenant_id FROM users WHERE id = $1), $2, $3, $4)
		`, userID, tokenHash, familyID, expiresAt)
		return err
	})
}

func (r *Repository) FindRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT user_id FROM refresh_tokens
			WHERE token_hash = $1
		`, tokenHash)
		return row.Scan(&userID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrExpiredToken
	}
	return userID, err
}

func (r *Repository) RotateRefreshToken(ctx context.Context, oldTokenHash, newTokenHash string, expiresAt time.Time) (string, error) {
	var userID string
	var replayed bool
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		var familyID string
		if err := tx.QueryRow(ctx, `SELECT family_id FROM refresh_tokens WHERE token_hash = $1`, oldTokenHash).Scan(&familyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, familyID); err != nil {
			return err
		}
		var revokedAt *time.Time
		var expired bool
		if err := tx.QueryRow(ctx, `
			SELECT user_id, revoked_at, expires_at <= NOW()
			FROM refresh_tokens
			WHERE token_hash = $1
			FOR UPDATE
		`, oldTokenHash).Scan(&userID, &revokedAt, &expired); err != nil {
			return err
		}
		if revokedAt != nil || expired {
			if _, err := tx.Exec(ctx, `
				UPDATE refresh_tokens
				SET revoked_at = COALESCE(revoked_at, NOW())
				WHERE family_id = $1
			`, familyID); err != nil {
				return err
			}
			replayed = true
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1`, oldTokenHash); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO refresh_tokens (user_id, tenant_id, token_hash, family_id, expires_at)
			SELECT user_id, tenant_id, $2, family_id, $3
			FROM refresh_tokens
			WHERE token_hash = $1
		`, oldTokenHash, newTokenHash, expiresAt)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrExpiredToken
	}
	if replayed {
		return "", domain.ErrExpiredToken
	}
	return userID, err
}

func (r *Repository) RevokeRefreshFamily(ctx context.Context, tokenHash string) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		var familyID string
		if err := tx.QueryRow(ctx, `SELECT family_id FROM refresh_tokens WHERE token_hash = $1`, tokenHash).Scan(&familyID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrExpiredToken
			}
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, familyID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE refresh_tokens
			SET revoked_at = COALESCE(revoked_at, NOW())
			WHERE family_id = $1
		`, familyID)
		return err
	})
}

func (r *Repository) RevokeUserSessions(ctx context.Context, userID string) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE refresh_tokens
			SET revoked_at = COALESCE(revoked_at, NOW())
			WHERE user_id = $1
		`, userID)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return domain.ErrNotFound
			}
		}
		return nil
	})
}

func (r *Repository) SavePasswordResetToken(ctx context.Context, tenantID, userID, tokenHash string, expiresAt time.Time) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE password_resets
			SET revoked_at = NOW()
			WHERE tenant_id = NULLIF($1, '') AND user_id = $2
			  AND used_at IS NULL AND revoked_at IS NULL
		`, tenantID, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO password_resets (tenant_id, user_id, token_hash, expires_at)
			VALUES ($1, $2, $3, $4)
		`, strPtr(tenantID), userID, tokenHash, expiresAt)
		return err
	})
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
		if _, err := tx.Exec(ctx, `
			UPDATE invites SET revoked_at = NOW()
			WHERE tenant_id = $1 AND email = LOWER(TRIM($2))
			  AND used_at IS NULL AND revoked_at IS NULL
		`, strPtr(tenantID), email); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO invites (tenant_id, email, role, permissions, token_hash, invited_by, expires_at)
			VALUES ($1, LOWER(TRIM($2)), $3, $4, $5, $6, $7)
		`, strPtr(tenantID), email, role, permissions, tokenHash, invitedBy, expiresAt)
		return err
	})
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
