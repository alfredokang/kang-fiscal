// Package dto é a forma como o módulo devolve o emitente para quem o chamou.
package dto

import "time"

// AddressDTO é o endereço fiscal.
type AddressDTO struct {
	Street, Number, Complement, District string
	CityCode, CityName, UF, ZipCode      string
	Phone                                string
}

// IssuerDTO é o emitente.
type IssuerDTO struct {
	ID                    string
	CNPJ                  string
	LegalName             string
	TradeName             string
	StateRegistration     string
	MunicipalRegistration string
	CNAE                  string
	TaxRegime             int
	Address               AddressDTO
	Environment           string
	CanIssue              bool
	Missing               []string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
