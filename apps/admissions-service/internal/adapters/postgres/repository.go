// Package postgres implements the authoritative Admissions repository.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/admissions-service/internal/domain"
	"github.com/auraedu/admissions-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct{ db *db.DB }

var _ ports.Repository = (*Repository)(nil)
var _ ports.CatalogueRepository = (*Repository)(nil)

func NewRepository(d *db.DB) *Repository { return &Repository{db: d} }
func tenantCtx(ctx context.Context, id string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: id})
}

const columns = `
	id, tenant_id, applicant_user_id, lead_id, programme_id, intake_id,
	programme_name, intake_name, legal_name, email, phone, answers, status,
	completion_percentage, missing_requirements, submitted_at, reviewed_by,
	reviewed_at, review_note, offer_status, offer_conditions, offer_expires_at,
	offer_issued_by, offer_accepted_at, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.Application, error) {
	var a domain.Application
	var answers []byte
	err := row.Scan(
		&a.ID, &a.TenantID, &a.ApplicantUserID, &a.LeadID, &a.ProgrammeID,
		&a.IntakeID, &a.ProgrammeName, &a.IntakeName, &a.LegalName, &a.Email,
		&a.Phone, &answers, &a.Status, &a.CompletionPercentage,
		&a.MissingRequirements, &a.SubmittedAt, &a.ReviewedBy, &a.ReviewedAt,
		&a.ReviewNote, &a.OfferStatus, &a.OfferConditions, &a.OfferExpiresAt,
		&a.OfferIssuedBy, &a.OfferAcceptedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == nil {
		err = json.Unmarshal(answers, &a.Answers)
	}
	return a, err
}
func (r *Repository) Create(ctx context.Context, a domain.Application) error {
	err := r.db.WithTx(tenantCtx(ctx, a.TenantID), func(ctx context.Context, tx pgx.Tx) error { return insertApplication(ctx, tx, a) })
	return mapConflict(err)
}
func insertApplication(ctx context.Context, tx pgx.Tx, a domain.Application) error {
	answers, err := json.Marshal(a.Answers)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO applications(`+columns+`) VALUES(
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,
		$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)`,
		a.ID, a.TenantID, a.ApplicantUserID, a.LeadID, a.ProgrammeID,
		a.IntakeID, a.ProgrammeName, a.IntakeName, a.LegalName, a.Email,
		a.Phone, answers, a.Status, a.CompletionPercentage, a.MissingRequirements,
		a.SubmittedAt, a.ReviewedBy, a.ReviewedAt, a.ReviewNote, a.OfferStatus,
		a.OfferConditions, a.OfferExpiresAt, a.OfferIssuedBy, a.OfferAcceptedAt,
		a.CreatedAt, a.UpdatedAt,
	)
	return err
}
func mapConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrConflict
		case "23503":
			return domain.ErrNotFound
		}
	}
	return err
}
func (r *Repository) Get(ctx context.Context, t, id string) (domain.Application, error) {
	var out domain.Application
	err := r.db.WithTx(tenantCtx(ctx, t), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM applications WHERE tenant_id=$1 AND id=$2`, t, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		out.Documents, err = documents(ctx, tx, t, out.ID)
		return err
	})
	return out, err
}
func documents(ctx context.Context, tx pgx.Tx, t, id string) ([]domain.Document, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, file_id, document_type, file_name, created_at
		FROM application_documents
		WHERE tenant_id=$1 AND application_id=$2
		ORDER BY created_at, id`, t, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Document{}
	for rows.Next() {
		var d domain.Document
		if err := rows.Scan(&d.ID, &d.FileID, &d.DocumentType, &d.FileName, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (r *Repository) List(ctx context.Context, t, applicant string, status domain.Status, limit int) ([]domain.Application, error) {
	out := []domain.Application{}
	err := r.db.WithTx(tenantCtx(ctx, t), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+columns+` FROM applications
			WHERE tenant_id=$1
				AND ($2='' OR applicant_user_id=$2)
				AND ($3='' OR status=$3)
			ORDER BY created_at DESC, id DESC LIMIT $4`,
			t, applicant, status, limit,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			a, err := scan(rows)
			if err != nil {
				return err
			}
			out = append(out, a)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		for i := range out {
			out[i].Documents, err = documents(ctx, tx, t, out[i].ID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}
func (r *Repository) Update(ctx context.Context, a domain.Application, expected domain.Status) error {
	return r.db.WithTx(tenantCtx(ctx, a.TenantID), func(ctx context.Context, tx pgx.Tx) error { return updateApplication(ctx, tx, a, expected) })
}
func updateApplication(ctx context.Context, tx pgx.Tx, a domain.Application, expected domain.Status) error {
	answers, err := json.Marshal(a.Answers)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE applications SET
		legal_name=$3, email=$4, phone=$5, answers=$6, status=$7,
		completion_percentage=$8, missing_requirements=$9, submitted_at=$10,
		reviewed_by=$11, reviewed_at=$12, review_note=$13, offer_status=$14,
		offer_conditions=$15, offer_expires_at=$16, offer_issued_by=$17,
		offer_accepted_at=$18, updated_at=$19
		WHERE tenant_id=$1 AND id=$2 AND status=$20`,
		a.TenantID, a.ID, a.LegalName, a.Email, a.Phone, answers, a.Status,
		a.CompletionPercentage, a.MissingRequirements, a.SubmittedAt,
		a.ReviewedBy, a.ReviewedAt, a.ReviewNote, a.OfferStatus,
		a.OfferConditions, a.OfferExpiresAt, a.OfferIssuedBy, a.OfferAcceptedAt,
		a.UpdatedAt, expected,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM application_documents WHERE tenant_id=$1 AND application_id=$2`, a.TenantID, a.ID); err != nil {
		return err
	}
	for _, d := range a.Documents {
		if _, err = tx.Exec(ctx, `INSERT INTO application_documents(
			id, tenant_id, application_id, file_id, document_type, file_name, created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7)`,
			d.ID, a.TenantID, a.ID, d.FileID, d.DocumentType, d.FileName, d.CreatedAt,
		); err != nil {
			return fmt.Errorf("admissions document: %w", err)
		}
	}
	return nil
}

func enqueue(ctx context.Context, tx pgx.Tx, tenantID, eventType string, payload map[string]any, createdAt time.Time) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO admissions_outbox(
		id, tenant_id, event_type, payload, created_at, next_attempt_at
	) VALUES($1,$2,$3,$4,$5,$5)`,
		uuid.NewString(), tenantID, eventType, encoded, createdAt,
	)
	return err
}
func (r *Repository) CreateWithEvent(ctx context.Context, a domain.Application, eventType string, payload map[string]any) error {
	err := r.db.WithTx(tenantCtx(ctx, a.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if err := insertApplication(ctx, tx, a); err != nil {
			return err
		}
		return enqueue(ctx, tx, a.TenantID, eventType, payload, a.CreatedAt)
	})
	return mapConflict(err)
}
func (r *Repository) UpdateWithEvent(ctx context.Context, a domain.Application, expected domain.Status, eventType string, payload map[string]any) error {
	return r.db.WithTx(tenantCtx(ctx, a.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if err := updateApplication(ctx, tx, a, expected); err != nil {
			return err
		}
		return enqueue(ctx, tx, a.TenantID, eventType, payload, a.UpdatedAt)
	})
}

const programmeColumns = `id,tenant_id,code,name,slug,summary,description,status,version,created_at,updated_at`
const intakeColumns = `
	id, tenant_id, programme_id, name, starts_at, application_opens_at,
	application_closes_at, capacity, status, version, created_at, updated_at`
const insertProgrammeSQL = `INSERT INTO admissions_programmes(` + programmeColumns + `)
	VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
const insertIntakeSQL = `INSERT INTO admissions_intakes(` + intakeColumns + `)
	VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
const updateProgrammeSQL = `UPDATE admissions_programmes
	SET code=$3, name=$4, slug=$5, summary=$6, description=$7,
		status=$8, version=$9, updated_at=$10
	WHERE tenant_id=$1 AND id=$2 AND version=$11`
const updateIntakeSQL = `UPDATE admissions_intakes
	SET name=$3, starts_at=$4, application_opens_at=$5,
		application_closes_at=$6, capacity=$7, status=$8, version=$9, updated_at=$10
	WHERE tenant_id=$1 AND id=$2 AND version=$11`

func programmeArgs(programme domain.Programme) []any {
	return []any{
		programme.ID, programme.TenantID, programme.Code, programme.Name,
		programme.Slug, programme.Summary, programme.Description, programme.Status,
		programme.Version, programme.CreatedAt, programme.UpdatedAt,
	}
}

func programmeUpdateArgs(programme domain.Programme, expectedVersion int) []any {
	return []any{
		programme.TenantID, programme.ID, programme.Code, programme.Name,
		programme.Slug, programme.Summary, programme.Description, programme.Status,
		programme.Version, programme.UpdatedAt, expectedVersion,
	}
}

func intakeArgs(intake domain.Intake) []any {
	return []any{
		intake.ID, intake.TenantID, intake.ProgrammeID, intake.Name, intake.StartsAt,
		intake.ApplicationOpensAt, intake.ApplicationClosesAt, intake.Capacity,
		intake.Status, intake.Version, intake.CreatedAt, intake.UpdatedAt,
	}
}

func intakeUpdateArgs(intake domain.Intake, expectedVersion int) []any {
	return []any{
		intake.TenantID, intake.ID, intake.Name, intake.StartsAt,
		intake.ApplicationOpensAt, intake.ApplicationClosesAt, intake.Capacity,
		intake.Status, intake.Version, intake.UpdatedAt, expectedVersion,
	}
}

func scanProgramme(row scanner) (domain.Programme, error) {
	var programme domain.Programme
	err := row.Scan(
		&programme.ID, &programme.TenantID, &programme.Code, &programme.Name,
		&programme.Slug, &programme.Summary, &programme.Description,
		&programme.Status, &programme.Version, &programme.CreatedAt,
		&programme.UpdatedAt,
	)
	programme.Intakes = []domain.Intake{}
	return programme, err
}

func scanIntake(row scanner) (domain.Intake, error) {
	var intake domain.Intake
	err := row.Scan(
		&intake.ID, &intake.TenantID, &intake.ProgrammeID, &intake.Name,
		&intake.StartsAt, &intake.ApplicationOpensAt, &intake.ApplicationClosesAt,
		&intake.Capacity, &intake.Status, &intake.Version, &intake.CreatedAt,
		&intake.UpdatedAt,
	)
	return intake, err
}

func (r *Repository) CreateProgramme(ctx context.Context, programme domain.Programme) error {
	err := r.db.WithTx(tenantCtx(ctx, programme.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, insertProgrammeSQL, programmeArgs(programme)...)
		return err
	})
	return mapConflict(err)
}

func (r *Repository) CreateProgrammeWithEvent(ctx context.Context, programme domain.Programme, eventType string, payload map[string]any) error {
	err := r.db.WithTx(tenantCtx(ctx, programme.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, insertProgrammeSQL, programmeArgs(programme)...); err != nil {
			return err
		}
		return enqueue(ctx, tx, programme.TenantID, eventType, payload, programme.CreatedAt)
	})
	return mapConflict(err)
}

func (r *Repository) GetProgramme(ctx context.Context, tenantID, id string) (domain.Programme, error) {
	var programme domain.Programme
	err := r.db.WithTx(tenantCtx(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		programme, err = scanProgramme(tx.QueryRow(ctx, `SELECT `+programmeColumns+` FROM admissions_programmes WHERE tenant_id=$1 AND id=$2`, tenantID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		programme.Intakes, err = queryIntakes(ctx, tx, tenantID, programme.ID, false, time.Time{})
		return err
	})
	return programme, err
}

func queryIntakes(ctx context.Context, tx pgx.Tx, tenantID, programmeID string, public bool, now time.Time) ([]domain.Intake, error) {
	rows, err := tx.Query(ctx, `SELECT `+intakeColumns+` FROM admissions_intakes
		WHERE tenant_id=$1 AND programme_id=$2
			AND (NOT $3 OR (
				status='open' AND application_opens_at<=$4 AND application_closes_at>$4
			))
		ORDER BY starts_at, id`,
		tenantID, programmeID, public, now.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Intake{}
	for rows.Next() {
		intake, err := scanIntake(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, intake)
	}
	return items, rows.Err()
}

func (r *Repository) ListProgrammes(ctx context.Context, tenantID string, public bool, now time.Time, limit int) ([]domain.Programme, error) {
	items := []domain.Programme{}
	err := r.db.WithTx(tenantCtx(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+programmeColumns+` FROM admissions_programmes
			WHERE tenant_id=$1 AND (NOT $2 OR status='published')
			ORDER BY name, id LIMIT $3`,
			tenantID, public, limit,
		)
		if err != nil {
			return err
		}
		for rows.Next() {
			programme, err := scanProgramme(rows)
			if err != nil {
				rows.Close()
				return err
			}
			items = append(items, programme)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		visible := items[:0]
		for i := range items {
			items[i].Intakes, err = queryIntakes(ctx, tx, tenantID, items[i].ID, public, now)
			if err != nil {
				return err
			}
			if !public || len(items[i].Intakes) > 0 {
				visible = append(visible, items[i])
			}
		}
		items = visible
		return nil
	})
	return items, err
}

func (r *Repository) UpdateProgramme(ctx context.Context, programme domain.Programme, expectedVersion int) error {
	err := r.db.WithTx(tenantCtx(ctx, programme.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, updateProgrammeSQL, programmeUpdateArgs(programme, expectedVersion)...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		return nil
	})
	return mapConflict(err)
}

func updateProgrammeRow(ctx context.Context, tx pgx.Tx, programme domain.Programme, expectedVersion int) error {
	tag, err := tx.Exec(ctx, updateProgrammeSQL, programmeUpdateArgs(programme, expectedVersion)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConflict
	}
	return nil
}

func (r *Repository) UpdateProgrammeWithEvent(
	ctx context.Context,
	programme domain.Programme,
	expectedVersion int,
	eventType string,
	payload map[string]any,
) error {
	err := r.db.WithTx(tenantCtx(ctx, programme.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if err := updateProgrammeRow(ctx, tx, programme, expectedVersion); err != nil {
			return err
		}
		return enqueue(ctx, tx, programme.TenantID, eventType, payload, programme.UpdatedAt)
	})
	return mapConflict(err)
}

func (r *Repository) CreateIntake(ctx context.Context, intake domain.Intake) error {
	err := r.db.WithTx(tenantCtx(ctx, intake.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, insertIntakeSQL, intakeArgs(intake)...)
		return err
	})
	return mapConflict(err)
}

func (r *Repository) CreateIntakeWithEvent(ctx context.Context, intake domain.Intake, eventType string, payload map[string]any) error {
	err := r.db.WithTx(tenantCtx(ctx, intake.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, insertIntakeSQL, intakeArgs(intake)...); err != nil {
			return err
		}
		return enqueue(ctx, tx, intake.TenantID, eventType, payload, intake.CreatedAt)
	})
	return mapConflict(err)
}

func (r *Repository) GetIntake(ctx context.Context, tenantID, id string) (domain.Intake, error) {
	var intake domain.Intake
	err := r.db.WithTx(tenantCtx(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		intake, err = scanIntake(tx.QueryRow(ctx, `SELECT `+intakeColumns+` FROM admissions_intakes WHERE tenant_id=$1 AND id=$2`, tenantID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	})
	return intake, err
}

func (r *Repository) UpdateIntake(ctx context.Context, intake domain.Intake, expectedVersion int) error {
	err := r.db.WithTx(tenantCtx(ctx, intake.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, updateIntakeSQL, intakeUpdateArgs(intake, expectedVersion)...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		return nil
	})
	return mapConflict(err)
}

func updateIntakeRow(ctx context.Context, tx pgx.Tx, intake domain.Intake, expectedVersion int) error {
	tag, err := tx.Exec(ctx, updateIntakeSQL, intakeUpdateArgs(intake, expectedVersion)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConflict
	}
	return nil
}

func (r *Repository) UpdateIntakeWithEvent(
	ctx context.Context,
	intake domain.Intake,
	expectedVersion int,
	eventType string,
	payload map[string]any,
) error {
	err := r.db.WithTx(tenantCtx(ctx, intake.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if err := updateIntakeRow(ctx, tx, intake, expectedVersion); err != nil {
			return err
		}
		return enqueue(ctx, tx, intake.TenantID, eventType, payload, intake.UpdatedAt)
	})
	return mapConflict(err)
}

func availableCatalogue(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, programmeID, intakeID string,
	now time.Time,
) (domain.Programme, domain.Intake, error) {
	programme, err := scanProgramme(tx.QueryRow(
		ctx,
		`SELECT `+programmeColumns+` FROM admissions_programmes
		 WHERE tenant_id=$1 AND id=$2 AND status='published'`,
		tenantID, programmeID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Programme{}, domain.Intake{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Programme{}, domain.Intake{}, err
	}
	intake, err := scanIntake(tx.QueryRow(
		ctx,
		`SELECT `+intakeColumns+` FROM admissions_intakes
		 WHERE tenant_id=$1 AND programme_id=$2 AND id=$3 AND status='open'
			AND application_opens_at<=$4 AND application_closes_at>$4
		 FOR SHARE`,
		tenantID, programmeID, intakeID, now.UTC(),
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Programme{}, domain.Intake{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Programme{}, domain.Intake{}, err
	}
	programme.Intakes = []domain.Intake{intake}
	return programme, intake, nil
}

func (r *Repository) ResolveAvailableIntake(
	ctx context.Context,
	tenantID, programmeID, intakeID string,
	now time.Time,
) (domain.Programme, domain.Intake, error) {
	var programme domain.Programme
	var intake domain.Intake
	err := r.db.WithTx(tenantCtx(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		programme, intake, err = availableCatalogue(ctx, tx, tenantID, programmeID, intakeID, now)
		return err
	})
	return programme, intake, err
}

func (r *Repository) CreateForAvailableIntake(
	ctx context.Context,
	application domain.Application,
	now time.Time,
	eventType string,
	payload map[string]any,
) error {
	err := r.db.WithTx(tenantCtx(ctx, application.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if _, _, err := availableCatalogue(
			ctx, tx, application.TenantID, application.ProgrammeID, application.IntakeID, now,
		); err != nil {
			return err
		}
		if err := insertApplication(ctx, tx, application); err != nil {
			return err
		}
		return enqueue(ctx, tx, application.TenantID, eventType, payload, application.CreatedAt)
	})
	return mapConflict(err)
}
func platformCtx(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__admissions_outbox__"})
}
func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := []ports.OutboxEvent{}
	err := r.db.WithTx(platformCtx(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE admissions_outbox
			SET attempts=attempts+1,
				next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN (
				SELECT id FROM admissions_outbox
				WHERE published_at IS NULL AND next_attempt_at<=now()
				ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
			)
			RETURNING id, tenant_id, event_type, payload, created_at`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e ports.OutboxEvent
			if err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
				return err
			}
			items = append(items, e)
		}
		return rows.Err()
	})
	return items, err
}
func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	return r.mark(ctx, id, "", true)
}
func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	return r.mark(ctx, id, message, false)
}
func (r *Repository) mark(ctx context.Context, id, message string, published bool) error {
	return r.db.WithTx(platformCtx(ctx), func(ctx context.Context, tx pgx.Tx) error {
		if published {
			_, err := tx.Exec(ctx, `UPDATE admissions_outbox SET published_at=$2,last_error=NULL WHERE id=$1`, id, time.Now().UTC())
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE admissions_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
	})
}
