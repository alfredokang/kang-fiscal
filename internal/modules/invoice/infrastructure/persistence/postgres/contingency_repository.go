package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
)

// ContingencyRepository é a implementação Postgres do estado de contingência.
type ContingencyRepository struct{ db *sql.DB }

// NewContingencyRepository monta o repositório.
func NewContingencyRepository(db *sql.DB) *ContingencyRepository {
	return &ContingencyRepository{db: db}
}

// Get lê o estado; sem linha, a autorizadora está normal.
func (r *ContingencyRepository) Get(ctx context.Context, uf string, model int, env string) (entity.Contingency, error) {
	c := entity.Contingency{UF: uf, Model: model, Environment: env}
	var started sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT active, kind, reason, started_at FROM invoice_contingency
		WHERE uf = $1 AND model = $2 AND environment = $3`, uf, model, env).Scan(&c.Active, &c.Kind, &c.Reason, &started)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	if started.Valid {
		c.StartedAt = started.Time
	}
	return c, err
}

// Activate liga a contingência. Ligar de novo uma já ativa mantém o início original: o
// dhCont das notas é a entrada em contingência, não a última confirmação.
func (r *ContingencyRepository) Activate(ctx context.Context, c entity.Contingency) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO invoice_contingency (uf, model, environment, active, kind, reason, started_at)
		VALUES ($1, $2, $3, TRUE, $4, $5, $6)
		ON CONFLICT (uf, model, environment) DO UPDATE SET
			kind = EXCLUDED.kind, reason = EXCLUDED.reason, active = TRUE,
			started_at = CASE WHEN invoice_contingency.active THEN invoice_contingency.started_at ELSE EXCLUDED.started_at END`,
		c.UF, c.Model, c.Environment, c.Kind, c.Reason, c.StartedAt)
	return err
}

// Deactivate volta a autorizadora ao normal.
func (r *ContingencyRepository) Deactivate(ctx context.Context, uf string, model int, env string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE invoice_contingency SET active = FALSE, failures = 0
		WHERE uf = $1 AND model = $2 AND environment = $3`, uf, model, env)
	return err
}

// RecordFailure soma a falha, recomeçando a contagem quando a anterior saiu da janela.
func (r *ContingencyRepository) RecordFailure(ctx context.Context, uf string, model int, env string, at time.Time, window time.Duration) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `INSERT INTO invoice_contingency (uf, model, environment, failures, last_failure_at)
		VALUES ($1, $2, $3, 1, $4)
		ON CONFLICT (uf, model, environment) DO UPDATE SET
			failures = CASE WHEN invoice_contingency.last_failure_at > $4 - make_interval(secs => $5)
			                THEN invoice_contingency.failures + 1 ELSE 1 END,
			last_failure_at = $4
		RETURNING failures`, uf, model, env, at, window.Seconds()).Scan(&n)
	return n, err
}

// ResetFailures zera a contagem depois de um envio bem-sucedido.
func (r *ContingencyRepository) ResetFailures(ctx context.Context, uf string, model int, env string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE invoice_contingency SET failures = 0
		WHERE uf = $1 AND model = $2 AND environment = $3 AND failures > 0`, uf, model, env)
	return err
}

// ListActive lista as contingências ligadas.
func (r *ContingencyRepository) ListActive(ctx context.Context) ([]entity.Contingency, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT uf, model, environment, kind, reason, started_at FROM invoice_contingency
		WHERE active ORDER BY uf LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.Contingency
	for rows.Next() {
		c := entity.Contingency{Active: true}
		var started sql.NullTime
		if err := rows.Scan(&c.UF, &c.Model, &c.Environment, &c.Kind, &c.Reason, &started); err != nil {
			return nil, err
		}
		c.StartedAt = started.Time
		out = append(out, c)
	}
	return out, rows.Err()
}
