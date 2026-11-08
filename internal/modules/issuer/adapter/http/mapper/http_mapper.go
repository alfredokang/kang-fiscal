// Package mapper converte entre HTTP e a aplicação.
package mapper

import (
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/adapter/http/request"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/adapter/http/response"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/dto"
)

// ToFiscalData converte o corpo em command.
func ToFiscalData(r request.FiscalDataRequest) command.FiscalData {
	a := r.Address
	return command.FiscalData{
		LegalName:             r.LegalName,
		TradeName:             r.TradeName,
		StateRegistration:     r.StateRegistration,
		MunicipalRegistration: r.MunicipalRegistration,
		CNAE:                  r.CNAE,
		TaxRegime:             r.TaxRegime,
		Address: command.Address{
			Street: a.Street, Number: a.Number, Complement: a.Complement, District: a.District,
			CityCode: a.CityCode, CityName: a.CityName, UF: a.UF, ZipCode: a.ZipCode, Phone: a.Phone,
		},
	}
}

// ToResponse converte o DTO em corpo.
func ToResponse(d dto.IssuerDTO) response.IssuerResponse {
	a := d.Address
	return response.IssuerResponse{
		ID:                    d.ID,
		CNPJ:                  d.CNPJ,
		LegalName:             d.LegalName,
		TradeName:             d.TradeName,
		StateRegistration:     d.StateRegistration,
		MunicipalRegistration: d.MunicipalRegistration,
		CNAE:                  d.CNAE,
		TaxRegime:             d.TaxRegime,
		Address: response.AddressResponse{
			Street: a.Street, Number: a.Number, Complement: a.Complement, District: a.District,
			CityCode: a.CityCode, CityName: a.CityName, UF: a.UF, ZipCode: a.ZipCode, Phone: a.Phone,
		},
		Environment: d.Environment,
		CanIssue:    d.CanIssue,
		Missing:     d.Missing,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}
