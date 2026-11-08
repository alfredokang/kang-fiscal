package factory_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/factory"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/valueobject"
)

var now = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func complete() factory.RawFiscalData {
	return factory.RawFiscalData{
		LegalName:         "  Café   Exemplo  LTDA ",
		StateRegistration: "123.456.789.110",
		TaxRegime:         3,
		Address: factory.RawAddress{
			Street: "Rua das Flores", Number: "100", District: "Centro",
			CityCode: "3550308", CityName: "São Paulo", UF: "sp", ZipCode: "01001-000",
		},
	}
}

func TestEmitenteNasceEmHomologacao(t *testing.T) {
	issuer, err := factory.NewIssuer("default", "11.222.333/0001-81", complete(), now)
	if err != nil {
		t.Fatal(err)
	}
	s := issuer.Snapshot()
	if s.Environment != vo.Homologation {
		t.Errorf("ambiente: %s", s.Environment)
	}
	if s.Data.LegalName != "Café Exemplo LTDA" {
		t.Errorf("razão social não foi limpa: %q", s.Data.LegalName)
	}
	if s.Data.Address.UF != "SP" || s.Data.Address.ZipCode != "01001000" {
		t.Errorf("endereço: %+v", s.Data.Address)
	}
	if !issuer.CanIssue() {
		t.Errorf("cadastro completo, mas falta: %v", issuer.Missing())
	}
}

func TestCadastroIncompletoEntraMasNaoVaiParaProducao(t *testing.T) {
	raw := complete()
	raw.StateRegistration = ""
	raw.Address.ZipCode = ""
	issuer, err := factory.NewIssuer("default", "11222333000181", raw, now)
	if err != nil {
		t.Fatal(err)
	}
	missing := issuer.Missing()
	if len(missing) != 2 || missing[0] != "state_registration" || missing[1] != "address.zip_code" {
		t.Errorf("faltando: %v", missing)
	}
	if err := issuer.PromoteToProduction(now); !errors.Is(err, domain.ErrIncompleteRegistration) {
		t.Errorf("promoção com cadastro incompleto: %v", err)
	}
}

func TestPromocaoParaProducao(t *testing.T) {
	issuer, _ := factory.NewIssuer("default", "11222333000181", complete(), now)
	if err := issuer.PromoteToProduction(now); err != nil {
		t.Fatal(err)
	}
	if issuer.Environment() != vo.Production {
		t.Errorf("ambiente: %s", issuer.Environment())
	}
	if err := issuer.PromoteToProduction(now); !errors.Is(err, domain.ErrAlreadyInProduction) {
		t.Errorf("segunda promoção: %v", err)
	}
}

func TestFactoryRecusa(t *testing.T) {
	cases := map[string]struct {
		cnpj   string
		mutate func(*factory.RawFiscalData)
		want   error
	}{
		"CNPJ com dígito errado": {"11222333000182", nil, domain.ErrInvalidCNPJ},
		"UF inexistente":         {"", func(r *factory.RawFiscalData) { r.Address.UF = "XX" }, domain.ErrInvalidUF},
		// 3106200 é Belo Horizonte: município de MG num endereço de SP.
		"cidade de outra UF": {"", func(r *factory.RawFiscalData) { r.Address.CityCode = "3106200" }, domain.ErrCityOutsideUF},
		"código IBGE curto":  {"", func(r *factory.RawFiscalData) { r.Address.CityCode = "355030" }, domain.ErrInvalidCityCode},
		"CEP curto":          {"", func(r *factory.RawFiscalData) { r.Address.ZipCode = "0100100" }, domain.ErrInvalidZipCode},
		"CRT 5":              {"", func(r *factory.RawFiscalData) { r.TaxRegime = 5 }, domain.ErrInvalidTaxRegime},
		"IE com letra":       {"", func(r *factory.RawFiscalData) { r.StateRegistration = "12A" }, domain.ErrInvalidStateReg},
		"sem razão social":   {"", func(r *factory.RawFiscalData) { r.LegalName = "   " }, domain.ErrRequiredField},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			raw := complete()
			if c.mutate != nil {
				c.mutate(&raw)
			}
			cnpj := c.cnpj
			if cnpj == "" {
				cnpj = "11222333000181"
			}
			if _, err := factory.NewIssuer("default", cnpj, raw, now); !errors.Is(err, c.want) {
				t.Errorf("esperava %v, veio %v", c.want, err)
			}
		})
	}
}

func TestIsentoECnpjAlfanumerico(t *testing.T) {
	raw := complete()
	raw.StateRegistration = "isento"
	issuer, err := factory.NewIssuer("default", "12.ABC.345/01DE-35", raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if s := issuer.Snapshot(); s.Data.StateRegistration != "ISENTO" || s.CNPJ != "12ABC34501DE35" {
		t.Errorf("IE=%s CNPJ=%s", s.Data.StateRegistration, s.CNPJ)
	}
}
