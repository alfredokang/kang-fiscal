// Package mapper converte entre o agregado e a linha do banco.
package mapper

import (
	"database/sql"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/entity"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/valueobject"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/infrastructure/persistence/postgres/model"
	"github.com/alfredokang/kang-fiscal/pkg/document"
)

// ToModel converte o agregado em linha.
func ToModel(i *entity.Issuer) model.IssuerModel {
	s := i.Snapshot()
	a := s.Data.Address
	return model.IssuerModel{
		ID:                    string(s.ID),
		TenantID:              s.TenantID,
		CNPJ:                  string(s.CNPJ),
		LegalName:             s.Data.LegalName,
		TradeName:             s.Data.TradeName,
		StateRegistration:     string(s.Data.StateRegistration),
		MunicipalRegistration: s.Data.MunicipalRegistration,
		CNAE:                  s.Data.CNAE,
		TaxRegime:             int(s.Data.TaxRegime),
		Street:                a.Street,
		Number:                a.Number,
		Complement:            a.Complement,
		District:              a.District,
		CityCode:              nullable(string(a.CityCode)),
		CityName:              a.CityName,
		UF:                    string(a.UF),
		ZipCode:               nullable(a.ZipCode),
		Phone:                 a.Phone,
		Environment:           string(s.Environment),
		CreatedAt:             s.CreatedAt,
		UpdatedAt:             s.UpdatedAt,
	}
}

// ToEntity reconstrói o agregado a partir da linha.
func ToEntity(m model.IssuerModel) *entity.Issuer {
	return entity.Restore(entity.Snapshot{
		ID:       vo.IssuerID(m.ID),
		TenantID: m.TenantID,
		CNPJ:     document.CNPJ(m.CNPJ),
		Data: entity.FiscalData{
			LegalName:             m.LegalName,
			TradeName:             m.TradeName,
			StateRegistration:     vo.StateRegistration(m.StateRegistration),
			MunicipalRegistration: m.MunicipalRegistration,
			CNAE:                  m.CNAE,
			TaxRegime:             vo.TaxRegime(m.TaxRegime),
			Address: entity.Address{
				Street: m.Street, Number: m.Number, Complement: m.Complement, District: m.District,
				CityCode: vo.CityCode(m.CityCode.String), CityName: m.CityName, UF: vo.UF(m.UF),
				ZipCode: m.ZipCode.String, Phone: m.Phone,
			},
		},
		Environment: vo.Environment(m.Environment),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	})
}

// nullable grava NULL em vez de texto vazio nas colunas CHAR: um CHAR(7) vazio vira sete
// espaços, e "  " passaria por código preenchido.
func nullable(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
