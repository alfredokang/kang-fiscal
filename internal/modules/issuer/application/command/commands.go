// Package command são as intenções de escrita do módulo.
package command

// Address é o endereço como veio da requisição.
type Address struct {
	Street, Number, Complement, District string
	CityCode, CityName, UF, ZipCode      string
	Phone                                string
}

// FiscalData são os dados corrigíveis do emitente.
type FiscalData struct {
	LegalName             string
	TradeName             string
	StateRegistration     string
	MunicipalRegistration string
	CNAE                  string
	TaxRegime             int
	Address               Address
}

// RegisterIssuer cadastra um emitente.
type RegisterIssuer struct {
	TenantID string
	CNPJ     string
	Data     FiscalData
}

// UpdateIssuer substitui os dados corrigíveis.
type UpdateIssuer struct {
	TenantID string
	IssuerID string
	Data     FiscalData
}

// PromoteIssuer passa o emitente para produção.
type PromoteIssuer struct {
	TenantID string
	IssuerID string
}
