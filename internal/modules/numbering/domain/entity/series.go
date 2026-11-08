// Package entity contém a série de numeração.
package entity

import (
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain"
)

// MaxNumber é o maior nNF do leiaute.
const MaxNumber = 999_999_999

// SeriesKey identifica uma sequência: cada emitente, modelo, série e ambiente numera
// separado. Homologação e produção não dividem número — a nota de teste não pode abrir
// buraco na numeração de verdade.
type SeriesKey struct {
	TenantID    string
	IssuerRef   string
	Model       int
	Series      int
	Environment string // homologation | production
}

// Validate confere a chave.
func (k SeriesKey) Validate() error {
	if k.TenantID == "" || k.IssuerRef == "" {
		return domain.ErrRequiredField
	}
	if k.Model != 55 && k.Model != 65 {
		return domain.ErrInvalidModel
	}
	// 890 a 899 são reservadas para a nota avulsa emitida pelo fisco.
	if k.Series < 0 || k.Series > 999 || (k.Series >= 890 && k.Series <= 899) {
		return domain.ErrInvalidSeries
	}
	if k.Environment != "homologation" && k.Environment != "production" {
		return domain.ErrRequiredField
	}
	return nil
}

// Reservation é um número entregue a um documento.
type Reservation struct {
	ID          string
	Key         SeriesKey
	Number      int
	DocumentRef string
	Status      string // reserved | used | voided
}

const (
	StatusReserved = "reserved"
	StatusUsed     = "used"
	StatusVoided   = "voided"
)
