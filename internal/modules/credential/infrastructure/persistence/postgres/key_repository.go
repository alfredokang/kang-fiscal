// Package postgres implementa as credenciais em SQL puro.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain/entity"
)

const columns = `id, tenant_id, name, prefix, hash, scopes, created_at, last_used_at, revoked_at`

// KeyRepository é a implementação Postgres.
type KeyRepository struct{ db *sql.DB }

// NewKeyRepository monta o repositório.
func NewKeyRepository(db *sql.DB) *KeyRepository { return &KeyRepository{db: db} }

// Create grava a credencial.
func (r *KeyRepository) Create(ctx context.Context, k *entity.Key) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO credential_keys (id, tenant_id, name, prefix, hash, scopes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, k.ID, k.TenantID, k.Name, k.Prefix, k.Hash, strings.Join(k.Scopes, ","), k.CreatedAt)
	return err
}

// FindByPrefix lê a credencial pelo prefixo público.
func (r *KeyRepository) FindByPrefix(ctx context.Context, prefix string) (*entity.Key, error) {
	k, err := scan(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM credential_keys WHERE prefix = $1`, prefix))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return k, err
}

// List lista as credenciais do tenant.
func (r *KeyRepository) List(ctx context.Context, tenantID string) ([]*entity.Key, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+` FROM credential_keys WHERE tenant_id = $1
		ORDER BY created_at DESC LIMIT 200`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*entity.Key
	for rows.Next() {
		k, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// Revoke revoga a credencial.
func (r *KeyRepository) Revoke(ctx context.Context, id string, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE credential_keys SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, id, at)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Touch registra o uso, no máximo uma vez por minuto: uma escrita por requisição faria
// cada consulta de nota virar um UPDATE.
func (r *KeyRepository) Touch(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE credential_keys SET last_used_at = $2
		WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < $2 - INTERVAL '1 minute')`, id, at)
	return err
}

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (*entity.Key, error) {
	var k entity.Key
	var scopes string
	var used, revoked sql.NullTime
	if err := s.Scan(&k.ID, &k.TenantID, &k.Name, &k.Prefix, &k.Hash, &scopes, &k.CreatedAt, &used, &revoked); err != nil {
		return nil, err
	}
	k.Scopes = strings.Split(scopes, ",")
	if used.Valid {
		t := used.Time
		k.LastUsedAt = &t
	}
	if revoked.Valid {
		t := revoked.Time
		k.RevokedAt = &t
	}
	return &k, nil
}
