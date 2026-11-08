// Package postgres implementa o repositório das notas em SQL puro.
package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

const columns = `id, tenant_id, issuer_ref, idempotency_key, request_hash, model, series, number,
	reservation_ref, environment, access_key, status, cstat, motive, protocol, total_amount::text,
	signed_xml, authorized_xml, COALESCE(xml_hash, ''), created_at, updated_at, authorized_at`

// InvoiceRepository é a implementação Postgres.
type InvoiceRepository struct{ db *sql.DB }

// NewInvoiceRepository monta o repositório.
func NewInvoiceRepository(db *sql.DB) *InvoiceRepository { return &InvoiceRepository{db: db} }

// Create grava a nota assinada.
func (r *InvoiceRepository) Create(ctx context.Context, i *entity.Invoice) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO invoice_invoices (id, tenant_id, issuer_ref, idempotency_key,
		request_hash, model, series, number, reservation_ref, environment, access_key, status, total_amount,
		signed_xml, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::numeric,$14,$15,$16)`,
		i.ID, i.TenantID, i.IssuerRef, i.IdempotencyKey, i.RequestHash, i.Model, i.Series, i.Number,
		i.ReservationRef, i.Environment, i.AccessKey, string(i.Status), i.TotalAmount, i.SignedXML, i.CreatedAt, i.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "invoice_invoices_idempotency" {
		return domain.ErrIdempotencyConflict
	}
	return err
}

// Update grava o resultado do envio. O gatilho do banco recusa mexer em nota autorizada.
func (r *InvoiceRepository) Update(ctx context.Context, i *entity.Invoice) error {
	var hash any
	if i.XMLHash != "" {
		hash = i.XMLHash
	}
	_, err := r.db.ExecContext(ctx, `UPDATE invoice_invoices SET status = $3, cstat = $4, motive = $5,
		protocol = $6, authorized_xml = $7, xml_hash = $8, updated_at = $9, authorized_at = $10
		WHERE tenant_id = $1 AND id = $2`,
		i.TenantID, i.ID, string(i.Status), i.CStat, i.Motive, i.Protocol, i.AuthorizedXML, hash, i.UpdatedAt, i.AuthorizedAt)
	return err
}

// UpdateDocument troca o XML assinado e a chave (nota que vai sair pela SVC).
func (r *InvoiceRepository) UpdateDocument(ctx context.Context, i *entity.Invoice) error {
	_, err := r.db.ExecContext(ctx, `UPDATE invoice_invoices SET access_key = $3, signed_xml = $4, updated_at = $5
		WHERE tenant_id = $1 AND id = $2 AND authorized_xml IS NULL`, i.TenantID, i.ID, i.AccessKey, i.SignedXML, i.UpdatedAt)
	return err
}

// LatestIssuer devolve o emitente da nota mais recente da UF (pelo cUF da chave).
func (r *InvoiceRepository) LatestIssuer(ctx context.Context, cUF string, model int, env string) (string, string, error) {
	var tenant, issuer string
	err := r.db.QueryRowContext(ctx, `SELECT tenant_id, issuer_ref FROM invoice_invoices
		WHERE left(access_key, 2) = $1 AND model = $2 AND environment = $3
		ORDER BY created_at DESC LIMIT 1`, cUF, model, env).Scan(&tenant, &issuer)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", domain.ErrNotFound
	}
	return tenant, issuer, err
}

// FindByIdempotencyKey lê a nota de uma Idempotency-Key.
func (r *InvoiceRepository) FindByIdempotencyKey(ctx context.Context, tenantID, key string) (*entity.Invoice, error) {
	return r.one(ctx, `WHERE tenant_id = $1 AND idempotency_key = $2`, tenantID, key)
}

// FindByAccessKey lê a nota pela chave de acesso.
func (r *InvoiceRepository) FindByAccessKey(ctx context.Context, tenantID, accessKey string) (*entity.Invoice, error) {
	return r.one(ctx, `WHERE tenant_id = $1 AND access_key = $2`, tenantID, accessKey)
}

// FindByID lê a nota pelo id.
func (r *InvoiceRepository) FindByID(ctx context.Context, tenantID, id string) (*entity.Invoice, error) {
	return r.one(ctx, `WHERE tenant_id = $1 AND id = $2`, tenantID, id)
}

// InRange lista as notas de uma série com número na faixa.
func (r *InvoiceRepository) InRange(ctx context.Context, k output.NumberKey, from, to int) ([]*entity.Invoice, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+` FROM invoice_invoices
		WHERE tenant_id = $1 AND issuer_ref = $2 AND environment = $3 AND model = $4 AND series = $5
		  AND number BETWEEN $6 AND $7 ORDER BY number LIMIT 1000`,
		k.TenantID, k.IssuerRef, k.Environment, k.Model, k.Series, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*entity.Invoice
	for rows.Next() {
		inv, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (r *InvoiceRepository) one(ctx context.Context, where string, args ...any) (*entity.Invoice, error) {
	inv, err := scan(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM invoice_invoices `+where, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return inv, err
}

// List lista as notas do tenant, mais recentes primeiro.
func (r *InvoiceRepository) List(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.Invoice, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoice_invoices WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := page.Limit
	if limit < 1 || limit > pagination.MaxLimit {
		limit = pagination.DefaultLimit
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+` FROM invoice_invoices WHERE tenant_id = $1
		ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, tenantID, limit, max(page.Offset(), 0))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*entity.Invoice
	for rows.Next() {
		inv, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, inv)
	}
	return out, total, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (*entity.Invoice, error) {
	var i entity.Invoice
	var status string
	var at sql.NullTime
	if err := s.Scan(&i.ID, &i.TenantID, &i.IssuerRef, &i.IdempotencyKey, &i.RequestHash, &i.Model, &i.Series,
		&i.Number, &i.ReservationRef, &i.Environment, &i.AccessKey, &status, &i.CStat, &i.Motive, &i.Protocol,
		&i.TotalAmount, &i.SignedXML, &i.AuthorizedXML, &i.XMLHash, &i.CreatedAt, &i.UpdatedAt, &at); err != nil {
		return nil, err
	}
	i.Status = entity.Status(status)
	if at.Valid {
		t := at.Time
		i.AuthorizedAt = &t
	}
	return &i, nil
}
