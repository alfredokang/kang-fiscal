// Package mapper converte entre o agregado, os commands e os DTOs.
package mapper

import (
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/entity"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/factory"
)

// ToDTO converte o agregado.
func ToDTO(i *entity.Issuer) dto.IssuerDTO {
	s := i.Snapshot()
	a := s.Data.Address
	missing := i.Missing()
	if missing == nil {
		missing = []string{}
	}
	return dto.IssuerDTO{
		ID:                    string(s.ID),
		CNPJ:                  string(s.CNPJ),
		LegalName:             s.Data.LegalName,
		TradeName:             s.Data.TradeName,
		StateRegistration:     string(s.Data.StateRegistration),
		MunicipalRegistration: s.Data.MunicipalRegistration,
		CNAE:                  s.Data.CNAE,
		TaxRegime:             int(s.Data.TaxRegime),
		Address: dto.AddressDTO{
			Street: a.Street, Number: a.Number, Complement: a.Complement, District: a.District,
			CityCode: string(a.CityCode), CityName: a.CityName, UF: string(a.UF),
			ZipCode: a.ZipCode, Phone: a.Phone,
		},
		Environment: string(s.Environment),
		CanIssue:    len(missing) == 0,
		Missing:     missing,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// ToRawFiscalData leva o command para a entrada da factory.
func ToRawFiscalData(d command.FiscalData) factory.RawFiscalData {
	return factory.RawFiscalData{
		LegalName:             d.LegalName,
		TradeName:             d.TradeName,
		StateRegistration:     d.StateRegistration,
		MunicipalRegistration: d.MunicipalRegistration,
		CNAE:                  d.CNAE,
		TaxRegime:             d.TaxRegime,
		Address: factory.RawAddress{
			Street: d.Address.Street, Number: d.Address.Number, Complement: d.Address.Complement,
			District: d.Address.District, CityCode: d.Address.CityCode, CityName: d.Address.CityName,
			UF: d.Address.UF, ZipCode: d.Address.ZipCode, Phone: d.Address.Phone,
		},
	}
}
