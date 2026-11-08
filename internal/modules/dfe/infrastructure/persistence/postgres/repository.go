// Package postgres implementa o recebimento de notas em SQL puro.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// Repository é a implementação Postgres.
type Repository struct{ db *sql.DB }

// NewRepository monta o repositório.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// Cursor lê o cursor; sem linha, começa do NSU zero.
func (r *Repository) Cursor(ctx context.Context, tenantID, issuerRef string) (entity.Cursor, error) {
	c := entity.Cursor{TenantID: tenantID, IssuerRef: issuerRef}
	err := r.db.QueryRowContext(ctx, `SELECT last_nsu, max_nsu, next_query_at, last_cstat FROM dfe_cursors
		WHERE tenant_id = $1 AND issuer_ref = $2`, tenantID, issuerRef).Scan(&c.LastNSU, &c.MaxNSU, &c.NextQueryAt, &c.LastCStat)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	return c, err
}

// SaveCursor grava o cursor.
func (r *Repository) SaveCursor(ctx context.Context, c entity.Cursor) error {
	next := c.NextQueryAt
	if next.IsZero() {
		next = time.Unix(0, 0)
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO dfe_cursors (tenant_id, issuer_ref, last_nsu, max_nsu, next_query_at, last_cstat)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, issuer_ref) DO UPDATE SET last_nsu = EXCLUDED.last_nsu, max_nsu = EXCLUDED.max_nsu,
			next_query_at = EXCLUDED.next_query_at, last_cstat = EXCLUDED.last_cstat`,
		c.TenantID, c.IssuerRef, c.LastNSU, c.MaxNSU, next, c.LastCStat)
	return err
}

// DueCursors lista os cursores com a espera vencida.
func (r *Repository) DueCursors(ctx context.Context, limit int) ([]entity.Cursor, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT tenant_id, issuer_ref, last_nsu, max_nsu, next_query_at, last_cstat
		FROM dfe_cursors WHERE next_query_at <= NOW() ORDER BY next_query_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.Cursor
	for rows.Next() {
		var c entity.Cursor
		if err := rows.Scan(&c.TenantID, &c.IssuerRef, &c.LastNSU, &c.MaxNSU, &c.NextQueryAt, &c.LastCStat); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveDocument grava o documento, ignorando o NSU repetido.
func (r *Repository) SaveDocument(ctx context.Context, d *entity.Document) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO dfe_documents (id, tenant_id, issuer_ref, nsu, schema, kind, access_key,
		emitter_doc, emitter_name, total, issued_at, situation, xml, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT DO NOTHING`,
		d.ID, d.TenantID, d.IssuerRef, d.NSU, d.Schema, string(d.Kind), d.AccessKey, d.EmitterDoc, d.EmitterName,
		d.Total, d.IssuedAt, d.Situation, d.XML, d.CreatedAt)
	return err
}

const docColumns = `id, tenant_id, issuer_ref, nsu, schema, kind, access_key, emitter_doc, emitter_name, total,
	issued_at, situation, manifestation, xml, created_at`

func scanDoc(s interface{ Scan(...any) error }) (*entity.Document, error) {
	var d entity.Document
	var kind string
	err := s.Scan(&d.ID, &d.TenantID, &d.IssuerRef, &d.NSU, &d.Schema, &kind, &d.AccessKey, &d.EmitterDoc,
		&d.EmitterName, &d.Total, &d.IssuedAt, &d.Situation, &d.Manifestation, &d.XML, &d.CreatedAt)
	d.Kind = entity.Kind(kind)
	return &d, err
}

// Find lê um documento do tenant.
func (r *Repository) Find(ctx context.Context, tenantID, id string) (*entity.Document, error) {
	d, err := scanDoc(r.db.QueryRowContext(ctx, `SELECT `+docColumns+` FROM dfe_documents WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return d, err
}

// List lista os documentos do emitente, os mais recentes primeiro.
func (r *Repository) List(ctx context.Context, tenantID, issuerRef string, page pagination.Params) ([]*entity.Document, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dfe_documents WHERE tenant_id = $1 AND issuer_ref = $2`,
		tenantID, issuerRef).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := page.Limit
	if limit < 1 || limit > pagination.MaxLimit {
		limit = pagination.DefaultLimit
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+docColumns+` FROM dfe_documents WHERE tenant_id = $1 AND issuer_ref = $2
		ORDER BY nsu DESC LIMIT $3 OFFSET $4`, tenantID, issuerRef, limit, max(page.Offset(), 0))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*entity.Document
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

// SetManifestation marca a manifestação em todos os documentos da chave.
func (r *Repository) SetManifestation(ctx context.Context, tenantID, accessKey, tpEvento string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE dfe_documents SET manifestation = $3 WHERE tenant_id = $1 AND access_key = $2`,
		tenantID, accessKey, tpEvento)
	return err
}
