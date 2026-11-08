// Package postgres implementa o repositório do certificado em SQL puro.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/entity"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/valueobject"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/infrastructure/persistence/postgres/mapper"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/infrastructure/persistence/postgres/model"
)

const publicColumns = `id, tenant_id, issuer_ref, holder_name, cnpj, ca_name, serial,
	fingerprint, not_before, not_after, created_at`

// CertificateRepository é a implementação Postgres.
type CertificateRepository struct {
	db *sql.DB
}

// NewCertificateRepository monta o repositório.
func NewCertificateRepository(db *sql.DB) *CertificateRepository {
	return &CertificateRepository{db: db}
}

// Replace aposenta o ativo (apagando o segredo) e grava o novo, numa transação: entre as
// duas operações o emitente nunca fica com dois ativos nem com nenhum.
func (r *CertificateRepository) Replace(ctx context.Context, c *entity.Certificate, sealed output.Sealed) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	m := mapper.ToModel(c)
	if _, err := tx.ExecContext(ctx, retireSQL, m.TenantID, m.IssuerRef, m.CreatedAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO certificate_certificates (`+publicColumns+`,
		sealed_pfx, sealed_password, active) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,TRUE)`,
		m.ID, m.TenantID, m.IssuerRef, m.HolderName, m.CNPJ, m.CAName, m.Serial, m.Fingerprint,
		m.NotBefore, m.NotAfter, m.CreatedAt, sealed.PFX, sealed.Password); err != nil {
		return err
	}
	return tx.Commit()
}

const retireSQL = `UPDATE certificate_certificates
	SET active = FALSE, sealed_pfx = NULL, sealed_password = NULL, retired_at = $3
	WHERE tenant_id = $1 AND issuer_ref = $2 AND active`

// Revoke aposenta o ativo sem pôr outro no lugar.
func (r *CertificateRepository) Revoke(ctx context.Context, tenantID string, issuer vo.IssuerRef, at time.Time) error {
	res, err := r.db.ExecContext(ctx, retireSQL, tenantID, string(issuer), at)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// FindActive lê os dados públicos do ativo.
func (r *CertificateRepository) FindActive(ctx context.Context, tenantID string, issuer vo.IssuerRef) (*entity.Certificate, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+publicColumns+` FROM certificate_certificates
		WHERE tenant_id = $1 AND issuer_ref = $2 AND active`, tenantID, string(issuer))
	m, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return mapper.ToEntity(m), nil
}

// FindActiveSealed lê o ativo com os segredos cifrados — só para assinar.
func (r *CertificateRepository) FindActiveSealed(ctx context.Context, tenantID string, issuer vo.IssuerRef) (*entity.Certificate, output.Sealed, error) {
	var m model.CertificateModel
	var s output.Sealed
	err := r.db.QueryRowContext(ctx, `SELECT `+publicColumns+`, sealed_pfx, sealed_password
		FROM certificate_certificates WHERE tenant_id = $1 AND issuer_ref = $2 AND active`,
		tenantID, string(issuer)).Scan(&m.ID, &m.TenantID, &m.IssuerRef, &m.HolderName, &m.CNPJ,
		&m.CAName, &m.Serial, &m.Fingerprint, &m.NotBefore, &m.NotAfter, &m.CreatedAt, &s.PFX, &s.Password)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, output.Sealed{}, domain.ErrNotFound
	}
	if err != nil {
		return nil, output.Sealed{}, err
	}
	return mapper.ToEntity(m), s, nil
}

func scan(row *sql.Row) (model.CertificateModel, error) {
	var m model.CertificateModel
	err := row.Scan(&m.ID, &m.TenantID, &m.IssuerRef, &m.HolderName, &m.CNPJ, &m.CAName,
		&m.Serial, &m.Fingerprint, &m.NotBefore, &m.NotAfter, &m.CreatedAt)
	return m, err
}
