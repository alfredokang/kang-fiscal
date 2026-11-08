// Package response são os corpos HTTP devolvidos pelo módulo.
package response

import (
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/adapter/http/request"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
)

// Profile é um perfil tributário.
type Profile struct {
	Code      string        `json:"code"`
	Name      string        `json:"name"`
	Rules     request.Rules `json:"rules"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// Rate é uma alíquota com vigência.
type Rate struct {
	ID        string        `json:"id"`
	Kind      string        `json:"kind"`
	UF        string        `json:"uf"`
	CityCode  string        `json:"city_code"`
	Value     decimal.Fixed `json:"value"`
	ValidFrom string        `json:"valid_from"`
	ValidTo   *string       `json:"valid_to"`
	Source    string        `json:"source"`
}

// Rates são as alíquotas resolvidas para uma data e local.
type Rates struct {
	CBS    decimal.Fixed `json:"cbs"`
	IBSUF  decimal.Fixed `json:"ibs_uf"`
	IBSMun decimal.Fixed `json:"ibs_mun"`
}

// Tax é um tributo calculado.
type Tax struct {
	Code  string         `json:"code"` // CST ou CSOSN
	Base  *decimal.Fixed `json:"base,omitempty"`
	Rate  *decimal.Fixed `json:"rate,omitempty"`
	Value *decimal.Fixed `json:"value,omitempty"`
}

// IBSCBS é o IBS/CBS calculado.
type IBSCBS struct {
	CST       string         `json:"cst"`
	ClassTrib string         `json:"class_trib"`
	Base      *decimal.Fixed `json:"base,omitempty"`
	IBSUF     *Tax           `json:"ibs_uf,omitempty"`
	IBSMun    *Tax           `json:"ibs_mun,omitempty"`
	CBS       *Tax           `json:"cbs,omitempty"`
	IBS       *decimal.Fixed `json:"ibs,omitempty"`
}

// Item é um item simulado.
type Item struct {
	CFOP   string        `json:"cfop"`
	VProd  decimal.Fixed `json:"v_prod"`
	ICMS   *Tax          `json:"icms,omitempty"`
	PIS    *Tax          `json:"pis,omitempty"`
	COFINS *Tax          `json:"cofins,omitempty"`
	IPI    *Tax          `json:"ipi,omitempty"`
	IBSCBS *IBSCBS       `json:"ibs_cbs,omitempty"`
}

// Totals são os totais da simulação.
type Totals struct {
	VProd   decimal.Fixed `json:"v_prod"`
	VDesc   decimal.Fixed `json:"v_desc"`
	VICMS   decimal.Fixed `json:"v_icms"`
	VIPI    decimal.Fixed `json:"v_ipi"`
	VPIS    decimal.Fixed `json:"v_pis"`
	VCOFINS decimal.Fixed `json:"v_cofins"`
	// VNF é o valor da nota pelos tributos atuais; em 2026, IBS e CBS não entram nele.
	VNF  decimal.Fixed  `json:"v_nf"`
	VIBS *decimal.Fixed `json:"v_ibs,omitempty"`
	VCBS *decimal.Fixed `json:"v_cbs,omitempty"`
	// VNFTot é o valor com IBS, CBS e IS — o que o consumidor vai pagar quando eles
	// deixarem de ser só destacados.
	VNFTot *decimal.Fixed `json:"v_nf_tot,omitempty"`
}

// Simulation é a resposta da simulação.
type Simulation struct {
	Rates  Rates  `json:"rates"`
	Items  []Item `json:"items"`
	Totals Totals `json:"totals"`
}
