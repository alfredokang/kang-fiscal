// Package request são os corpos HTTP aceitos pela emissão. Números são decimal.Fixed:
// chegam como "33.33" ou 33.33 e nunca passam por float.
package request

import "github.com/alfredokang/kang-fiscal/pkg/decimal"

// IssueRequest é o POST /v1/nfe.
type IssueRequest struct {
	IssuerID       string    `json:"issuer_id"`
	Model          int       `json:"model"`  // 55
	Series         int       `json:"series"` // padrão 1
	NatureOfOp     string    `json:"nature_of_operation"`
	Purpose        int       `json:"purpose"`  // finNFe, padrão 1
	Presence       int       `json:"presence"` // indPres, padrão 2 (internet)
	FinalConsumer  bool      `json:"final_consumer"`
	Recipient      Recipient `json:"recipient"`
	Items          []Item    `json:"items"`
	Payments       []Payment `json:"payments"`
	FreightMode    *int      `json:"freight_mode"` // modFrete, padrão 9 (sem frete)
	AdditionalInfo string    `json:"additional_info"`
}

// Recipient é o destinatário.
type Recipient struct {
	CNPJ        string  `json:"cnpj"`
	CPF         string  `json:"cpf"`
	Name        string  `json:"name"`
	IE          string  `json:"ie"`
	IEIndicator int     `json:"ie_indicator"` // 1 contribuinte, 2 isento, 9 não contribuinte
	Email       string  `json:"email"`
	Address     Address `json:"address"`
}

// Address é um endereço.
type Address struct {
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

// Item é um item.
type Item struct {
	Code        string        `json:"code"`
	Description string        `json:"description"`
	NCM         string        `json:"ncm"`
	CEST        string        `json:"cest"`
	GTIN        string        `json:"gtin"`
	Unit        string        `json:"unit"`
	Quantity    decimal.Fixed `json:"quantity"`
	UnitPrice   decimal.Fixed `json:"unit_price"`
	Discount    decimal.Fixed `json:"discount"`
	Freight     decimal.Fixed `json:"freight"`
	Insurance   decimal.Fixed `json:"insurance"`
	Other       decimal.Fixed `json:"other"`
	Origin      int           `json:"origin"`
	TaxProfile  string        `json:"tax_profile"`
}

// Payment é uma forma de pagamento.
type Payment struct {
	Method string        `json:"method"` // tPag: 01 dinheiro, 03 crédito, 17 PIX...
	Amount decimal.Fixed `json:"amount"`
}
