// Package postgres implementa o repositório do emitente em SQL puro.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/entity"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/valueobject"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/infrastructure/persistence/postgres/mapper"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/infrastructure/persistence/postgres/model"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

const columns = `id, tenant_id, cnpj, legal_name, trade_name, state_registration,
	municipal_registration, cnae, tax_regime, street, number, complement, district,
	city_code, city_name, uf, zip_code, phone, environment, created_at, updated_at`

// sortable é a allowlist de ordenação. Interpolar coluna sem allowlist é injeção de SQL
// pela porta dos fundos.
var sortable = map[string]string{
	"created_at": "created_at",
	"legal_name": "legal_name",
}

// SortFields são os campos que a rota de lista aceita em ?sort, o primeiro é o padrão.
var SortFields = []string{"created_at", "legal_name"}

// IssuerRepository é a implementação Postgres.
type IssuerRepository struct {
	db *sql.DB
}

// NewIssuerRepository monta o repositório.
func NewIssuerRepository(db *sql.DB) *IssuerRepository {
	return &IssuerRepository{db: db}
}

// Create grava um emitente novo.
func (r *IssuerRepository) Create(ctx context.Context, issuer *entity.Issuer) error {
	m := mapper.ToModel(issuer)
	_, err := r.db.ExecContext(ctx, `INSERT INTO issuer_issuers (`+columns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		m.ID, m.TenantID, m.CNPJ, m.LegalName, m.TradeName, m.StateRegistration,
		m.MunicipalRegistration, m.CNAE, m.TaxRegime, m.Street, m.Number, m.Complement, m.District,
		m.CityCode, m.CityName, m.UF, m.ZipCode, m.Phone, m.Environment, m.CreatedAt, m.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrIssuerAlreadyExists
	}
	return err
}

// Update grava os dados corrigíveis e o ambiente. CNPJ, tenant e criação não mudam.
func (r *IssuerRepository) Update(ctx context.Context, issuer *entity.Issuer) error {
	m := mapper.ToModel(issuer)
	res, err := r.db.ExecContext(ctx, `UPDATE issuer_issuers SET
		legal_name = $3, trade_name = $4, state_registration = $5, municipal_registration = $6,
		cnae = $7, tax_regime = $8, street = $9, number = $10, complement = $11, district = $12,
		city_code = $13, city_name = $14, uf = $15, zip_code = $16, phone = $17,
		environment = $18, updated_at = $19
		WHERE id = $1 AND tenant_id = $2`,
		m.ID, m.TenantID, m.LegalName, m.TradeName, m.StateRegistration, m.MunicipalRegistration,
		m.CNAE, m.TaxRegime, m.Street, m.Number, m.Complement, m.District,
		m.CityCode, m.CityName, m.UF, m.ZipCode, m.Phone, m.Environment, m.UpdatedAt)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrIssuerNotFound
	}
	return nil
}

// FindByID lê um emitente do tenant. Id de outro tenant é "não encontrado", nunca
// "proibido": dizer que existe já é vazar.
func (r *IssuerRepository) FindByID(ctx context.Context, tenantID string, id vo.IssuerID) (*entity.Issuer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+columns+` FROM issuer_issuers WHERE id = $1 AND tenant_id = $2`, string(id), tenantID)
	m, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrIssuerNotFound
	}
	if err != nil {
		return nil, err
	}
	return mapper.ToEntity(m), nil
}

// List lista os emitentes do tenant, paginado.
func (r *IssuerRepository) List(ctx context.Context, tenantID string, p pagination.Params) ([]*entity.Issuer, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM issuer_issuers WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}

	sortCol, ok := sortable[p.Sort]
	if !ok {
		sortCol = "created_at"
	}
	order := "DESC"
	if p.Order == "asc" {
		order = "ASC"
	}
	limit := p.Limit
	if limit < 1 || limit > pagination.MaxLimit {
		limit = pagination.DefaultLimit
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT %s FROM issuer_issuers WHERE tenant_id = $1 ORDER BY %s %s, id %s LIMIT $2 OFFSET $3`,
		columns, sortCol, order, order), tenantID, limit, max(p.Offset(), 0))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*entity.Issuer
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, mapper.ToEntity(m))
	}
	return out, total, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (model.IssuerModel, error) {
	var m model.IssuerModel
	err := s.Scan(&m.ID, &m.TenantID, &m.CNPJ, &m.LegalName, &m.TradeName, &m.StateRegistration,
		&m.MunicipalRegistration, &m.CNAE, &m.TaxRegime, &m.Street, &m.Number, &m.Complement,
		&m.District, &m.CityCode, &m.CityName, &m.UF, &m.ZipCode, &m.Phone, &m.Environment,
		&m.CreatedAt, &m.UpdatedAt)
	return m, err
}
