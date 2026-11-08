package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
)

func d(s string) decimal.Fixed {
	v, _ := decimal.ParseAuto(s)
	return v
}

func date(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestPerfilValidaNoCadastro(t *testing.T) {
	rules := tax.Profile{CFOPInternal: "5102", CFOPInterstate: "6102",
		ICMS: tax.ICMSRule{Code: "00", Rate: d("18")}, PIS: tax.PISCOFINSRule{CST: "01", Rate: d("1.65")},
		COFINS: tax.PISCOFINSRule{CST: "01", Rate: d("7.6")}}
	if _, err := entity.NewProfile("default", "venda-cafe-sp", "Venda de café em SP", rules, time.Now()); err != nil {
		t.Fatal(err)
	}
	bad := rules
	bad.ICMS.Code = "10" // ST com retenção: o motor ainda não calcula, então o cadastro recusa
	if _, err := entity.NewProfile("default", "x1", "X", bad, time.Now()); !errors.Is(err, domain.ErrInvalidProfile) {
		t.Errorf("CST que o motor não calcula: %v", err)
	}
	bad = rules
	bad.CFOPInternal = "510"
	if _, err := entity.NewProfile("default", "x1", "X", bad, time.Now()); !errors.Is(err, domain.ErrInvalidProfile) {
		t.Errorf("CFOP curto: %v", err)
	}
	if _, err := entity.NewProfile("default", "Venda Café", "X", rules, time.Now()); !errors.Is(err, domain.ErrInvalidCode) {
		t.Errorf("código com espaço: %v", err)
	}
}

func TestResolveAliquotaPorVigencia(t *testing.T) {
	end := date("2026-12-31")
	mk := func(kind entity.RateKind, uf, city, v string, from string, to *time.Time) *entity.Rate {
		r, err := entity.NewRate(kind, uf, city, d(v), date(from), to, "LC 214/2025")
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	rates := []*entity.Rate{
		mk(entity.CBS, "", "", "0.9", "2026-01-01", &end),
		mk(entity.IBSUF, "", "", "0.1", "2026-01-01", &end),
		mk(entity.IBSMun, "", "", "0", "2026-01-01", &end),
		// Um município que fixou alíquota própria prevalece sobre a geral.
		mk(entity.IBSMun, "", "3550308", "0.05", "2026-06-01", &end),
	}
	r, err := entity.Resolve(rates, date("2026-11-08"), "SP", "3550308")
	if err != nil || r.CBS.String() != "0.9000" || r.IBSUF.String() != "0.1000" || r.IBSMun.String() != "0.0500" {
		t.Errorf("SP capital: %+v %v", r, err)
	}
	r, _ = entity.Resolve(rates, date("2026-11-08"), "MG", "3106200")
	if r.IBSMun.String() != "0.0000" {
		t.Errorf("município sem alíquota própria usa a geral: %s", r.IBSMun)
	}
	// 2027 não está cadastrado: a emissão precisa parar, não inventar.
	if _, err := entity.Resolve(rates, date("2027-01-02"), "SP", "3550308"); !errors.Is(err, domain.ErrNoRate) {
		t.Errorf("sem alíquota para 2027: %v", err)
	}
	if _, err := entity.NewRate(entity.CBS, "", "", d("101"), date("2026-01-01"), nil, "x"); !errors.Is(err, domain.ErrInvalidRate) {
		t.Errorf("alíquota acima de 100: %v", err)
	}
}
