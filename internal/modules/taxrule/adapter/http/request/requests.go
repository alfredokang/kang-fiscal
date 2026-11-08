// Package request são os corpos HTTP aceitos pelo módulo. Números chegam como string
// ("18.00") ou número JSON e nunca passam por float (decimal.Fixed lê o texto).
package request

import "github.com/alfredokang/kang-fiscal/pkg/decimal"

// ProfileRequest é o PUT /v1/admin/tax-profiles/{code}.
type ProfileRequest struct {
	Name  string `json:"name"`
	Rules Rules  `json:"rules"`
}

// Rules é a regra fiscal.
type Rules struct {
	CFOPInternal           string  `json:"cfop_internal"`
	CFOPInterstate         string  `json:"cfop_interstate"`
	ICMS                   ICMS    `json:"icms"`
	PIS                    PIS     `json:"pis"`
	COFINS                 PIS     `json:"cofins"`
	IPI                    *IPI    `json:"ipi"`
	IBSCBS                 *IBSCBS `json:"ibs_cbs"`
	ExcludeICMSFromPISBase bool    `json:"exclude_icms_from_pis_base"`
}

// ICMS é a regra do ICMS: CST (2 dígitos) no regime normal, CSOSN (3) no Simples.
type ICMS struct {
	Code              string        `json:"code"`
	Rate              decimal.Fixed `json:"rate"`
	BaseReduction     decimal.Fixed `json:"base_reduction"`
	FCPRate           decimal.Fixed `json:"fcp_rate"`
	SimplesCreditRate decimal.Fixed `json:"simples_credit_rate"`
}

// PIS é a regra do PIS ou da COFINS.
type PIS struct {
	CST  string        `json:"cst"`
	Rate decimal.Fixed `json:"rate"`
}

// IPI é a regra do IPI.
type IPI struct {
	CST       string        `json:"cst"`
	Rate      decimal.Fixed `json:"rate"`
	LegalCode string        `json:"legal_code"`
}

// IBSCBS é a regra do IBS/CBS. As alíquotas não vêm aqui: vêm da tabela por vigência.
type IBSCBS struct {
	CST           string        `json:"cst"`
	ClassTrib     string        `json:"class_trib"`
	RateReduction decimal.Fixed `json:"rate_reduction"`
	NoTax         bool          `json:"no_tax"`
}

// RateRequest é o POST /v1/admin/tax-rates.
type RateRequest struct {
	Kind      string        `json:"kind"` // cbs, ibs_uf, ibs_mun
	UF        string        `json:"uf"`
	CityCode  string        `json:"city_code"`
	Value     decimal.Fixed `json:"value"`
	ValidFrom string        `json:"valid_from"` // AAAA-MM-DD
	ValidTo   string        `json:"valid_to"`
	Source    string        `json:"source"`
}

// SimulateRequest é o POST /v1/admin/tax/simulate.
type SimulateRequest struct {
	Date       string         `json:"date"` // vazio = hoje
	UF         string         `json:"uf"`
	CityCode   string         `json:"city_code"`
	Interstate bool           `json:"interstate"`
	Items      []SimulateItem `json:"items"`
}

// SimulateItem é um item a simular.
type SimulateItem struct {
	Profile   string        `json:"profile"`
	Origin    int           `json:"origin"`
	Quantity  decimal.Fixed `json:"quantity"`
	UnitPrice decimal.Fixed `json:"unit_price"`
	Discount  decimal.Fixed `json:"discount"`
	Freight   decimal.Fixed `json:"freight"`
	Insurance decimal.Fixed `json:"insurance"`
	Other     decimal.Fixed `json:"other"`
}
