// Package usecase orquestra os perfis, as alíquotas e a simulação.
package usecase

import (
	"context"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// TaxRuleService é a fachada do módulo.
type TaxRuleService struct {
	profiles output.ProfileRepository
	rates    output.RateRepository
	now      func() time.Time
}

// NewTaxRuleService monta o serviço.
func NewTaxRuleService(p output.ProfileRepository, r output.RateRepository) *TaxRuleService {
	return &TaxRuleService{profiles: p, rates: r, now: func() time.Time { return time.Now().UTC() }}
}

// SaveProfile cria ou substitui um perfil.
//
// Substituir não muda nota já emitida: a nota guarda o snapshot do perfil que usou.
func (s *TaxRuleService) SaveProfile(ctx context.Context, tenantID, code, name string, rules tax.Profile) (*entity.Profile, error) {
	p, err := entity.NewProfile(tenantID, code, name, rules, s.now())
	if err != nil {
		return nil, err
	}
	return p, s.profiles.Save(ctx, p)
}

// GetProfile lê um perfil.
func (s *TaxRuleService) GetProfile(ctx context.Context, tenantID, code string) (*entity.Profile, error) {
	return s.profiles.Find(ctx, tenantID, code)
}

// ListProfiles lista os perfis do tenant.
func (s *TaxRuleService) ListProfiles(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.Profile, int64, error) {
	return s.profiles.List(ctx, tenantID, page)
}

// AddRate cadastra uma alíquota com vigência.
func (s *TaxRuleService) AddRate(ctx context.Context, kind entity.RateKind, uf, city string, value decimal.Fixed,
	from time.Time, to *time.Time, source string) (*entity.Rate, error) {
	r, err := entity.NewRate(kind, uf, city, value, from, to, source)
	if err != nil {
		return nil, err
	}
	return r, s.rates.Add(ctx, r)
}

// ListRates lista as alíquotas cadastradas.
func (s *TaxRuleService) ListRates(ctx context.Context, page pagination.Params) ([]*entity.Rate, int64, error) {
	return s.rates.List(ctx, page)
}

// ResolveRates devolve as alíquotas de IBS/CBS vigentes na data e no local.
func (s *TaxRuleService) ResolveRates(ctx context.Context, date time.Time, uf, city string) (tax.Rates, error) {
	candidates, err := s.rates.Candidates(ctx, date, uf, city)
	if err != nil {
		return tax.Rates{}, err
	}
	return entity.Resolve(candidates, date, uf, city)
}

// SimulationItem é um item a simular.
type SimulationItem struct {
	ProfileCode string
	Item        tax.Item
}

// Simulation é o pedido de simulação.
type Simulation struct {
	TenantID   string
	Date       time.Time
	UF         string // UF do fato gerador (do emitente, na venda presencial)
	CityCode   string
	Interstate bool
	Items      []SimulationItem
}

// SimulationResult são os itens calculados e os totais.
type SimulationResult struct {
	Rates tax.Rates
	Items []tax.Result
	Total nfe.Total
}

// Simulate calcula os tributos sem emitir nada.
//
// É o diferencial para o e-commerce: o carrinho mostra os tributos com o mesmo motor que vai
// emitir a nota, e o valor que o cliente vê é o que vai para o XML.
func (s *TaxRuleService) Simulate(ctx context.Context, q Simulation) (SimulationResult, error) {
	if len(q.Items) == 0 || len(q.Items) > 990 {
		return SimulationResult{}, domain.ErrRequiredField
	}
	rates, err := s.ResolveRates(ctx, q.Date, q.UF, q.CityCode)
	if err != nil {
		return SimulationResult{}, err
	}
	out := SimulationResult{Rates: rates}
	profiles := map[string]*entity.Profile{}
	var dets []nfe.Det
	for i, it := range q.Items {
		p, ok := profiles[it.ProfileCode]
		if !ok {
			if p, err = s.profiles.Find(ctx, q.TenantID, it.ProfileCode); err != nil {
				return SimulationResult{}, err
			}
			profiles[it.ProfileCode] = p
		}
		res, err := tax.Compute(it.Item, p.Rules, rates, tax.Context{Simples: p.IsSimples(), Interstate: q.Interstate})
		if err != nil {
			return SimulationResult{}, err
		}
		out.Items = append(out.Items, res)
		prod := nfe.Prod{VProd: res.VProd, IndTot: 1}
		for _, pair := range []struct {
			dst **nfe.Dec
			v   nfe.Dec
		}{{&prod.VDesc, it.Item.Discount}, {&prod.VFrete, it.Item.Freight}, {&prod.VSeg, it.Item.Insurance}, {&prod.VOutro, it.Item.Other}} {
			if !pair.v.IsZero() {
				*pair.dst = pair.v.Ptr()
			}
		}
		dets = append(dets, nfe.Det{NItem: i + 1, Prod: prod, Imposto: res.Imposto})
	}
	if out.Total, err = nfe.ComputeTotals(dets, true); err != nil {
		return SimulationResult{}, err
	}
	return out, nil
}
