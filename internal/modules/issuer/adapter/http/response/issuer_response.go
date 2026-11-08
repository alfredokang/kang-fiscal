// Package response são os corpos HTTP devolvidos pelo módulo.
package response

import "time"

// AddressResponse é o endereço fiscal.
type AddressResponse struct {
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

// IssuerResponse é o emitente.
type IssuerResponse struct {
	ID                    string          `json:"id"`
	CNPJ                  string          `json:"cnpj"`
	LegalName             string          `json:"legal_name"`
	TradeName             string          `json:"trade_name"`
	StateRegistration     string          `json:"state_registration"`
	MunicipalRegistration string          `json:"municipal_registration"`
	CNAE                  string          `json:"cnae"`
	TaxRegime             int             `json:"tax_regime"`
	Address               AddressResponse `json:"address"`
	Environment           string          `json:"environment"`
	// CanIssue e Missing dizem de uma vez o que falta, em vez de a pessoa descobrir pela
	// rejeição da SEFAZ, uma de cada vez.
	CanIssue  bool      `json:"can_issue"`
	Missing   []string  `json:"missing"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
