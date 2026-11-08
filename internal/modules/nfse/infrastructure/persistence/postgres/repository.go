// Package postgres implementa as NFS-e em SQL puro.
package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

const columns = `id, tenant_id, issuer_ref, idempotency_key, request_hash, series, number, reservation_ref, environment,
	dps_id, COALESCE(access_key, ''), nfse_number, status, error_code, message, total::text, signed_xml, nfse_xml,
	COALESCE(xml_hash, ''), created_at, updated_at`

// Repository é a implementação Postgres.
type Repository struct{ db *sql.DB }

// NewRepository monta o repositório.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// Create grava a DPS assinada.
func (r *Repository) Create(ctx context.Context, s *entity.ServiceInvoice) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO nfse_documents (id, tenant_id, issuer_ref, idempotency_key, request_hash,
		series, number, reservation_ref, environment, dps_id, status, total, signed_xml, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::numeric,$13,$14,$15)`,
		s.ID, s.TenantID, s.IssuerRef, s.IdempotencyKey, s.RequestHash, s.Series, s.Number, s.ReservationRef,
		s.Environment, s.DPSID, string(s.Status), s.Total, s.SignedXML, s.CreatedAt, s.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "nfse_documents_idempotency" {
		return domain.ErrIdempotencyConflict
	}
	return err
}

// Update grava o resultado. O gatilho recusa mexer em NFS-e autorizada.
func (r *Repository) Update(ctx context.Context, s *entity.ServiceInvoice) error {
	var key, hash any
	if s.AccessKey != "" {
		key = s.AccessKey
	}
	if s.XMLHash != "" {
		hash = s.XMLHash
	}
	_, err := r.db.ExecContext(ctx, `UPDATE nfse_documents SET access_key = $3, nfse_number = $4, status = $5,
		error_code = $6, message = $7, nfse_xml = $8, xml_hash = $9, updated_at = $10 WHERE tenant_id = $1 AND id = $2`,
		s.TenantID, s.ID, key, s.NFSeNumber, string(s.Status), s.ErrorCode, s.Message, s.NFSeXML, hash, s.UpdatedAt)
	return err
}

func scan(row interface{ Scan(...any) error }) (*entity.ServiceInvoice, error) {
	var s entity.ServiceInvoice
	var status string
	err := row.Scan(&s.ID, &s.TenantID, &s.IssuerRef, &s.IdempotencyKey, &s.RequestHash, &s.Series, &s.Number,
		&s.ReservationRef, &s.Environment, &s.DPSID, &s.AccessKey, &s.NFSeNumber, &status, &s.ErrorCode, &s.Message,
		&s.Total, &s.SignedXML, &s.NFSeXML, &s.XMLHash, &s.CreatedAt, &s.UpdatedAt)
	s.Status = entity.Status(status)
	return &s, err
}

func (r *Repository) one(ctx context.Context, where string, args ...any) (*entity.ServiceInvoice, error) {
	s, err := scan(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM nfse_documents `+where, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return s, err
}

// FindByIdempotencyKey lê pela Idempotency-Key.
func (r *Repository) FindByIdempotencyKey(ctx context.Context, tenantID, key string) (*entity.ServiceInvoice, error) {
	return r.one(ctx, `WHERE tenant_id = $1 AND idempotency_key = $2`, tenantID, key)
}

// FindByKey lê pela chave de acesso ou pelo Id da DPS.
func (r *Repository) FindByKey(ctx context.Context, tenantID, key string) (*entity.ServiceInvoice, error) {
	return r.one(ctx, `WHERE tenant_id = $1 AND (access_key = $2 OR dps_id = $2)`, tenantID, key)
}

// List lista as NFS-e do tenant.
func (r *Repository) List(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.ServiceInvoice, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nfse_documents WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := page.Limit
	if limit < 1 || limit > pagination.MaxLimit {
		limit = pagination.DefaultLimit
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+` FROM nfse_documents WHERE tenant_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, tenantID, limit, max(page.Offset(), 0))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*entity.ServiceInvoice
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}
