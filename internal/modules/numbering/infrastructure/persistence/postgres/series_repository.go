// Package postgres implementa a numeração em SQL puro.
package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain/entity"
)

// SeriesRepository é a implementação Postgres.
type SeriesRepository struct{ db *sql.DB }

// NewSeriesRepository monta o repositório.
func NewSeriesRepository(db *sql.DB) *SeriesRepository { return &SeriesRepository{db: db} }

// Reserve entrega o próximo número sob lock da linha da série.
func (r *SeriesRepository) Reserve(ctx context.Context, k entity.SeriesKey, documentRef string, at time.Time) (entity.Reservation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Reservation{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// A série nasce no primeiro uso. ON CONFLICT DO NOTHING + FOR UPDATE: com duas
	// transações chegando juntas, uma cria e a outra espera o lock da linha criada.
	if _, err := tx.ExecContext(ctx, `INSERT INTO numbering_series (tenant_id, issuer_ref, model, series, environment, next_number)
		VALUES ($1, $2, $3, $4, $5, 1) ON CONFLICT DO NOTHING`, k.TenantID, k.IssuerRef, k.Model, k.Series, k.Environment); err != nil {
		return entity.Reservation{}, err
	}
	var next int
	if err := tx.QueryRowContext(ctx, `SELECT next_number FROM numbering_series
		WHERE tenant_id = $1 AND issuer_ref = $2 AND model = $3 AND series = $4 AND environment = $5 FOR UPDATE`,
		k.TenantID, k.IssuerRef, k.Model, k.Series, k.Environment).Scan(&next); err != nil {
		return entity.Reservation{}, err
	}
	if next > entity.MaxNumber {
		return entity.Reservation{}, domain.ErrExhausted
	}
	id, err := uuid.NewV7()
	if err != nil {
		return entity.Reservation{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO numbering_reservations
		(id, tenant_id, issuer_ref, model, series, environment, number, document_ref, reserved_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id.String(), k.TenantID, k.IssuerRef, k.Model, k.Series, k.Environment, next, documentRef, at); err != nil {
		return entity.Reservation{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE numbering_series SET next_number = next_number + 1
		WHERE tenant_id = $1 AND issuer_ref = $2 AND model = $3 AND series = $4 AND environment = $5`,
		k.TenantID, k.IssuerRef, k.Model, k.Series, k.Environment); err != nil {
		return entity.Reservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return entity.Reservation{}, err
	}
	return entity.Reservation{ID: id.String(), Key: k, Number: next, DocumentRef: documentRef, Status: entity.StatusReserved}, nil
}

// SetNext ajusta o próximo número, sob o mesmo lock da reserva.
func (r *SeriesRepository) SetNext(ctx context.Context, k entity.SeriesKey, next int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO numbering_series (tenant_id, issuer_ref, model, series, environment, next_number)
		VALUES ($1, $2, $3, $4, $5, 1) ON CONFLICT DO NOTHING`, k.TenantID, k.IssuerRef, k.Model, k.Series, k.Environment); err != nil {
		return err
	}
	var current int
	if err := tx.QueryRowContext(ctx, `SELECT next_number FROM numbering_series
		WHERE tenant_id = $1 AND issuer_ref = $2 AND model = $3 AND series = $4 AND environment = $5 FOR UPDATE`,
		k.TenantID, k.IssuerRef, k.Model, k.Series, k.Environment).Scan(&current); err != nil {
		return err
	}
	var maxReserved sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(number) FROM numbering_reservations
		WHERE tenant_id = $1 AND issuer_ref = $2 AND model = $3 AND series = $4 AND environment = $5`,
		k.TenantID, k.IssuerRef, k.Model, k.Series, k.Environment).Scan(&maxReserved); err != nil {
		return err
	}
	if maxReserved.Valid && int64(next) <= maxReserved.Int64 {
		return domain.ErrNumberBackward
	}
	if _, err := tx.ExecContext(ctx, `UPDATE numbering_series SET next_number = $6
		WHERE tenant_id = $1 AND issuer_ref = $2 AND model = $3 AND series = $4 AND environment = $5`,
		k.TenantID, k.IssuerRef, k.Model, k.Series, k.Environment, next); err != nil {
		return err
	}
	return tx.Commit()
}

// Settle fecha uma reserva aberta.
func (r *SeriesRepository) Settle(ctx context.Context, tenantID, id, status string, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE numbering_reservations SET status = $3, settled_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'reserved'`, tenantID, id, status, at)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// PendingVoid lista as reservas abertas mais antigas que o corte.
func (r *SeriesRepository) PendingVoid(ctx context.Context, before time.Time, limit int) ([]entity.Reservation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, tenant_id, issuer_ref, model, series, environment, number, document_ref
		FROM numbering_reservations WHERE status = 'reserved' AND reserved_at < $1
		ORDER BY reserved_at LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.Reservation
	for rows.Next() {
		var res entity.Reservation
		if err := rows.Scan(&res.ID, &res.Key.TenantID, &res.Key.IssuerRef, &res.Key.Model, &res.Key.Series,
			&res.Key.Environment, &res.Number, &res.DocumentRef); err != nil {
			return nil, err
		}
		res.Status = entity.StatusReserved
		out = append(out, res)
	}
	return out, rows.Err()
}
