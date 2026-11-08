package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// VoidRepository é a implementação Postgres das inutilizações.
type VoidRepository struct{ db *sql.DB }

// NewVoidRepository monta o repositório.
func NewVoidRepository(db *sql.DB) *VoidRepository { return &VoidRepository{db: db} }

const voidColumns = `id, tenant_id, issuer_ref, environment, model, series, number_from, number_to, justification,
	reservation_refs, status, cstat, motive, protocol, signed_xml, proc_xml, created_at, updated_at`

// CreateVoid grava o pedido assinado, antes do envio.
func (r *VoidRepository) CreateVoid(ctx context.Context, v *entity.Void) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO invoice_voids (id, tenant_id, issuer_ref, environment, model, series,
		number_from, number_to, justification, reservation_refs, status, signed_xml, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		v.ID, v.TenantID, v.IssuerRef, v.Environment, v.Model, v.Series, v.From, v.To, v.Justification,
		strings.Join(v.ReservationRefs, ","), string(v.Status), v.SignedXML, v.CreatedAt, v.UpdatedAt)
	return err
}

// UpdateVoid grava o resultado. O gatilho recusa mexer em inutilização homologada.
func (r *VoidRepository) UpdateVoid(ctx context.Context, v *entity.Void) error {
	_, err := r.db.ExecContext(ctx, `UPDATE invoice_voids SET status = $3, cstat = $4, motive = $5, protocol = $6,
		proc_xml = $7, updated_at = $8 WHERE tenant_id = $1 AND id = $2`,
		v.TenantID, v.ID, string(v.Status), v.CStat, v.Motive, v.Protocol, v.ProcXML, v.UpdatedAt)
	return err
}

// ListVoids lista as inutilizações do tenant.
func (r *VoidRepository) ListVoids(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.Void, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoice_voids WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := page.Limit
	if limit < 1 || limit > pagination.MaxLimit {
		limit = pagination.DefaultLimit
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+voidColumns+` FROM invoice_voids WHERE tenant_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, tenantID, limit, max(page.Offset(), 0))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*entity.Void
	for rows.Next() {
		var v entity.Void
		var status, refs string
		if err := rows.Scan(&v.ID, &v.TenantID, &v.IssuerRef, &v.Environment, &v.Model, &v.Series, &v.From, &v.To,
			&v.Justification, &refs, &status, &v.CStat, &v.Motive, &v.Protocol, &v.SignedXML, &v.ProcXML,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, 0, err
		}
		v.Status = entity.EventStatus(status)
		if refs != "" {
			v.ReservationRefs = strings.Split(refs, ",")
		}
		out = append(out, &v)
	}
	return out, total, rows.Err()
}
