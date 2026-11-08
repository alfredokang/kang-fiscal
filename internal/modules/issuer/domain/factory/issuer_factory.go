// Package factory transforma dados crus em agregado válido.
package factory

import (
	"strings"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/entity"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/valueobject"
)

// RawAddress é o endereço como chegou de fora.
type RawAddress struct {
	Street, Number, Complement, District string
	CityCode, CityName, UF, ZipCode      string
	Phone                                string
}

// RawFiscalData são os dados corrigíveis como chegaram de fora.
type RawFiscalData struct {
	LegalName             string
	TradeName             string
	StateRegistration     string
	MunicipalRegistration string
	CNAE                  string
	TaxRegime             int
	Address               RawAddress
}

// NewIssuer valida tudo e cria o emitente em homologação.
func NewIssuer(tenantID, cnpj string, raw RawFiscalData, now time.Time) (*entity.Issuer, error) {
	doc, err := vo.ParseCNPJ(cnpj)
	if err != nil {
		return nil, err
	}
	data, err := ParseFiscalData(raw)
	if err != nil {
		return nil, err
	}
	id, err := vo.NewIssuerID()
	if err != nil {
		return nil, err
	}
	return entity.New(id, tenantID, doc, data, now)
}

// ParseFiscalData passa cada campo pelo value object dele.
//
// Os campos de endereço são opcionais aqui e cobrados na emissão (entity.Missing). Os que
// vierem preenchidos, porém, precisam estar certos: aceitar um CEP de 7 dígitos agora é
// adiar a rejeição para a primeira nota.
func ParseFiscalData(raw RawFiscalData) (entity.FiscalData, error) {
	uf, err := vo.ParseUF(raw.Address.UF)
	if err != nil {
		return entity.FiscalData{}, err
	}
	regime, err := vo.ParseTaxRegime(raw.TaxRegime)
	if err != nil {
		return entity.FiscalData{}, err
	}
	ie, err := vo.ParseStateRegistration(raw.StateRegistration)
	if err != nil {
		return entity.FiscalData{}, err
	}

	addr := entity.Address{
		Street:     clean(raw.Address.Street),
		Number:     clean(raw.Address.Number),
		Complement: clean(raw.Address.Complement),
		District:   clean(raw.Address.District),
		CityName:   clean(raw.Address.CityName),
		UF:         uf,
		Phone:      digitsOnly(raw.Address.Phone),
	}
	if strings.TrimSpace(raw.Address.CityCode) != "" {
		if addr.CityCode, err = vo.ParseCityCode(raw.Address.CityCode, uf); err != nil {
			return entity.FiscalData{}, err
		}
	}
	if strings.TrimSpace(raw.Address.ZipCode) != "" {
		if addr.ZipCode, err = vo.ParseZipCode(raw.Address.ZipCode); err != nil {
			return entity.FiscalData{}, err
		}
	}

	return entity.FiscalData{
		LegalName:             clean(raw.LegalName),
		TradeName:             clean(raw.TradeName),
		StateRegistration:     ie,
		MunicipalRegistration: clean(raw.MunicipalRegistration),
		CNAE:                  digitsOnly(raw.CNAE),
		TaxRegime:             regime,
		Address:               addr,
	}, nil
}

// clean junta espaços repetidos. A SEFAZ rejeita espaço duplo e espaço nas pontas em
// campo de texto, e é o que o copiar-e-colar do cartão CNPJ mais traz.
func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
