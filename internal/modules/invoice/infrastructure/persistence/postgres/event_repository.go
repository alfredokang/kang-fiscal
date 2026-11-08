package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
)

const eventColumns = `id, tenant_id, invoice_id, access_key, type, seq, status, cstat, motive, protocol,
	text, signed_xml, proc_xml, created_at, updated_at`

// EventRepository é a implementação Postgres dos eventos.
type EventRepository struct{ db *sql.DB }

// NewEventRepository monta o repositório.
func NewEventRepository(db *sql.DB) *EventRepository { return &EventRepository{db: db} }

// CreateEvent grava o evento assinado, antes do envio.
func (r *EventRepository) CreateEvent(ctx context.Context, e *entity.Event) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO invoice_events (id, tenant_id, invoice_id, access_key, type, seq,
		status, text, signed_xml, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		e.ID, e.TenantID, e.InvoiceID, e.AccessKey, e.Type, e.Seq, string(e.Status), e.Text, e.SignedXML, e.CreatedAt, e.UpdatedAt)
	return err
}

// UpdateEvent grava o resultado. O gatilho recusa mexer em evento registrado.
func (r *EventRepository) UpdateEvent(ctx context.Context, e *entity.Event) error {
	_, err := r.db.ExecContext(ctx, `UPDATE invoice_events SET status = $3, cstat = $4, motive = $5, protocol = $6,
		proc_xml = $7, updated_at = $8 WHERE tenant_id = $1 AND id = $2`,
		e.TenantID, e.ID, string(e.Status), e.CStat, e.Motive, e.Protocol, e.ProcXML, e.UpdatedAt)
	return err
}

// Events lista os eventos da nota.
func (r *EventRepository) Events(ctx context.Context, tenantID, accessKey string) ([]*entity.Event, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+eventColumns+` FROM invoice_events
		WHERE tenant_id = $1 AND access_key = $2 ORDER BY created_at, id LIMIT 100`, tenantID, accessKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*entity.Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// FindEvent lê um evento do tenant.
func (r *EventRepository) FindEvent(ctx context.Context, tenantID, id string) (*entity.Event, error) {
	e, err := scanEvent(r.db.QueryRowContext(ctx, `SELECT `+eventColumns+` FROM invoice_events
		WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return e, err
}

func scanEvent(s scanner) (*entity.Event, error) {
	var e entity.Event
	var status string
	if err := s.Scan(&e.ID, &e.TenantID, &e.InvoiceID, &e.AccessKey, &e.Type, &e.Seq, &status, &e.CStat, &e.Motive,
		&e.Protocol, &e.Text, &e.SignedXML, &e.ProcXML, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	e.Status = entity.EventStatus(status)
	return &e, nil
}
