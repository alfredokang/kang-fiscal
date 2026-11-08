// Package postgres implementa os repositórios do taxrule em SQL puro.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// ProfileRepository é a implementação Postgres dos perfis.
type ProfileRepository struct{ db *sql.DB }

// NewProfileRepository monta o repositório.
func NewProfileRepository(db *sql.DB) *ProfileRepository { return &ProfileRepository{db: db} }

// Save grava ou substitui o perfil.
func (r *ProfileRepository) Save(ctx context.Context, p *entity.Profile) error {
	rules, err := json.Marshal(toRulesJSON(p.Rules))
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO taxrule_profiles (tenant_id, code, name, rules, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, code) DO UPDATE SET name = EXCLUDED.name, rules = EXCLUDED.rules, updated_at = EXCLUDED.updated_at`,
		p.TenantID, p.Code, p.Name, rules, p.UpdatedAt)
	return err
}

// Find lê um perfil do tenant.
func (r *ProfileRepository) Find(ctx context.Context, tenantID, code string) (*entity.Profile, error) {
	row := r.db.QueryRowContext(ctx, `SELECT tenant_id, code, name, rules, updated_at FROM taxrule_profiles
		WHERE tenant_id = $1 AND code = $2`, tenantID, code)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrProfileNotFound
	}
	return p, err
}

// List lista os perfis do tenant em ordem de código.
func (r *ProfileRepository) List(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.Profile, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM taxrule_profiles WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT tenant_id, code, name, rules, updated_at FROM taxrule_profiles
		WHERE tenant_id = $1 ORDER BY code LIMIT $2 OFFSET $3`, tenantID, limit(page), max(page.Offset(), 0))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*entity.Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanProfile(s scanner) (*entity.Profile, error) {
	var p entity.Profile
	var raw []byte
	if err := s.Scan(&p.TenantID, &p.Code, &p.Name, &raw, &p.UpdatedAt); err != nil {
		return nil, err
	}
	var rj rulesJSON
	if err := json.Unmarshal(raw, &rj); err != nil {
		return nil, err
	}
	p.Rules = rj.toProfile()
	return &p, nil
}

// RateRepository é a implementação Postgres das alíquotas.
type RateRepository struct{ db *sql.DB }

// NewRateRepository monta o repositório.
func NewRateRepository(db *sql.DB) *RateRepository { return &RateRepository{db: db} }

const rateColumns = `id, kind, uf, city_code, value::text, valid_from, valid_to, source`

// Add grava a alíquota. A sobreposição é barrada pela restrição de exclusão do banco.
func (r *RateRepository) Add(ctx context.Context, rate *entity.Rate) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO taxrule_rates (id, kind, uf, city_code, value, valid_from, valid_to, source)
		VALUES ($1, $2, $3, $4, $5::numeric, $6, $7, $8)`,
		rate.ID, string(rate.Kind), rate.UF, rate.CityCode, rate.Value.String(), rate.ValidFrom, rate.ValidTo, rate.Source)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23P01" { // exclusion_violation
		return domain.ErrRateOverlap
	}
	return err
}

// Candidates devolve as alíquotas vigentes na data, gerais ou do local pedido.
func (r *RateRepository) Candidates(ctx context.Context, date time.Time, uf, city string) ([]*entity.Rate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+rateColumns+` FROM taxrule_rates
		WHERE valid_from <= $1 AND (valid_to IS NULL OR valid_to >= $1)
		  AND uf IN ('', $2) AND city_code IN ('', $3)
		ORDER BY kind LIMIT 50`, date.Format("2006-01-02"), uf, city)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRates(rows)
}

// List lista as alíquotas, as mais recentes primeiro.
func (r *RateRepository) List(ctx context.Context, page pagination.Params) ([]*entity.Rate, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM taxrule_rates`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+rateColumns+` FROM taxrule_rates
		ORDER BY valid_from DESC, kind, uf, city_code LIMIT $1 OFFSET $2`, limit(page), max(page.Offset(), 0))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out, err := scanRates(rows)
	return out, total, err
}

func scanRates(rows *sql.Rows) ([]*entity.Rate, error) {
	var out []*entity.Rate
	for rows.Next() {
		var rt entity.Rate
		var kind, value string
		var to sql.NullTime
		if err := rows.Scan(&rt.ID, &kind, &rt.UF, &rt.CityCode, &value, &rt.ValidFrom, &to, &rt.Source); err != nil {
			return nil, err
		}
		v, err := decimal.Parse(value, 4)
		if err != nil {
			return nil, err
		}
		rt.Kind, rt.Value = entity.RateKind(kind), v
		if to.Valid {
			t := to.Time
			rt.ValidTo = &t
		}
		out = append(out, &rt)
	}
	return out, rows.Err()
}

func limit(p pagination.Params) int {
	if p.Limit < 1 || p.Limit > pagination.MaxLimit {
		return pagination.DefaultLimit
	}
	return p.Limit
}

// rulesJSON é a forma persistida do perfil. As tags de JSON ficam aqui, na infraestrutura:
// o tax.Profile é do pkg/ e não conhece o banco.
type rulesJSON struct {
	CFOPInternal   string   `json:"cfop_internal"`
	CFOPInterstate string   `json:"cfop_interstate"`
	ICMS           icmsJSON `json:"icms"`
	PIS            pisJSON  `json:"pis"`
	COFINS         pisJSON  `json:"cofins"`
	IPI            *ipiJSON `json:"ipi,omitempty"`
	IBSCBS         *ibsJSON `json:"ibs_cbs,omitempty"`
	ExcludeICMS    bool     `json:"exclude_icms_from_pis_base"`
}

type icmsJSON struct {
	Code              string        `json:"code"`
	Rate              decimal.Fixed `json:"rate"`
	BaseReduction     decimal.Fixed `json:"base_reduction"`
	FCPRate           decimal.Fixed `json:"fcp_rate"`
	SimplesCreditRate decimal.Fixed `json:"simples_credit_rate"`
}

type pisJSON struct {
	CST  string        `json:"cst"`
	Rate decimal.Fixed `json:"rate"`
}

type ipiJSON struct {
	CST       string        `json:"cst"`
	Rate      decimal.Fixed `json:"rate"`
	LegalCode string        `json:"legal_code"`
}

type ibsJSON struct {
	CST           string        `json:"cst"`
	ClassTrib     string        `json:"class_trib"`
	RateReduction decimal.Fixed `json:"rate_reduction"`
	NoTax         bool          `json:"no_tax"`
}

func toRulesJSON(p tax.Profile) rulesJSON {
	r := rulesJSON{
		CFOPInternal: p.CFOPInternal, CFOPInterstate: p.CFOPInterstate, ExcludeICMS: p.ExcludeICMSFromPISBase,
		ICMS: icmsJSON{Code: p.ICMS.Code, Rate: p.ICMS.Rate, BaseReduction: p.ICMS.BaseReduction,
			FCPRate: p.ICMS.FCPRate, SimplesCreditRate: p.ICMS.SimplesCreditRate},
		PIS:    pisJSON{CST: p.PIS.CST, Rate: p.PIS.Rate},
		COFINS: pisJSON{CST: p.COFINS.CST, Rate: p.COFINS.Rate},
	}
	if p.IPI != nil {
		r.IPI = &ipiJSON{CST: p.IPI.CST, Rate: p.IPI.Rate, LegalCode: p.IPI.LegalCode}
	}
	if p.IBSCBS != nil {
		r.IBSCBS = &ibsJSON{CST: p.IBSCBS.CST, ClassTrib: p.IBSCBS.ClassTrib,
			RateReduction: p.IBSCBS.RateReduction, NoTax: p.IBSCBS.NoTax}
	}
	return r
}

func (r rulesJSON) toProfile() tax.Profile {
	p := tax.Profile{
		CFOPInternal: r.CFOPInternal, CFOPInterstate: r.CFOPInterstate, ExcludeICMSFromPISBase: r.ExcludeICMS,
		ICMS: tax.ICMSRule{Code: r.ICMS.Code, Rate: r.ICMS.Rate, BaseReduction: r.ICMS.BaseReduction,
			FCPRate: r.ICMS.FCPRate, SimplesCreditRate: r.ICMS.SimplesCreditRate},
		PIS:    tax.PISCOFINSRule{CST: r.PIS.CST, Rate: r.PIS.Rate},
		COFINS: tax.PISCOFINSRule{CST: r.COFINS.CST, Rate: r.COFINS.Rate},
	}
	if r.IPI != nil {
		p.IPI = &tax.IPIRule{CST: r.IPI.CST, Rate: r.IPI.Rate, LegalCode: r.IPI.LegalCode}
	}
	if r.IBSCBS != nil {
		p.IBSCBS = &tax.IBSCBSRule{CST: r.IBSCBS.CST, ClassTrib: r.IBSCBS.ClassTrib,
			RateReduction: r.IBSCBS.RateReduction, NoTax: r.IBSCBS.NoTax}
	}
	return p
}
