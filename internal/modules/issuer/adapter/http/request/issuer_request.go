// Package request são os corpos HTTP aceitos pelo módulo.
package request

// AddressRequest é o endereço fiscal.
type AddressRequest struct {
	Street     string `json:"street"`
	Number     string `json:"number"`
	Complement string `json:"complement"`
	District   string `json:"district"`
	CityCode   string `json:"city_code"`
	CityName   string `json:"city_name"`
	UF         string `json:"uf"`
	ZipCode    string `json:"zip_code"`
	Phone      string `json:"phone"`
}

// FiscalDataRequest são os dados corrigíveis.
type FiscalDataRequest struct {
	LegalName             string         `json:"legal_name"`
	TradeName             string         `json:"trade_name"`
	StateRegistration     string         `json:"state_registration"`
	MunicipalRegistration string         `json:"municipal_registration"`
	CNAE                  string         `json:"cnae"`
	TaxRegime             int            `json:"tax_regime"`
	Address               AddressRequest `json:"address"`
}

// RegisterIssuerRequest é o POST /v1/admin/issuers.
type RegisterIssuerRequest struct {
	CNPJ string `json:"cnpj"`
	FiscalDataRequest
}
