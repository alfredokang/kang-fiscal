// Package model é a linha de issuer_issuers, sem tags (Constituição, Art. 4.3).
package model

import (
	"database/sql"
	"time"
)

// IssuerModel espelha issuer_issuers coluna a coluna.
type IssuerModel struct {
	ID                    string         // id
	TenantID              string         // tenant_id
	CNPJ                  string         // cnpj
	LegalName             string         // legal_name
	TradeName             string         // trade_name
	StateRegistration     string         // state_registration
	MunicipalRegistration string         // municipal_registration
	CNAE                  string         // cnae
	TaxRegime             int            // tax_regime
	Street                string         // street
	Number                string         // number
	Complement            string         // complement
	District              string         // district
	CityCode              sql.NullString // city_code
	CityName              string         // city_name
	UF                    string         // uf
	ZipCode               sql.NullString // zip_code
	Phone                 string         // phone
	Environment           string         // environment
	CreatedAt             time.Time      // created_at
	UpdatedAt             time.Time      // updated_at
}
