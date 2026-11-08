package entity

import "time"

// Contingency é o estado de contingência de uma autorizadora.
type Contingency struct {
	UF          string
	Model       int
	Environment string
	Active      bool
	Kind        string // SVCAN | SVCRS
	Reason      string
	StartedAt   time.Time
}

// Regras da ativação automática.
const (
	// FailuresToActivate: falhas de conexão seguidas que ativam a SVC.
	FailuresToActivate = 3
	// FailureWindow: as falhas precisam cair nesta janela; uma falha isolada de manhã e
	// outra à tarde não são a SEFAZ fora do ar.
	FailureWindow = 10 * time.Minute
)

// EmissionType é o tpEmis da contingência: 6 = SVC-AN, 7 = SVC-RS.
func (c Contingency) EmissionType() int {
	if c.Kind == "SVCRS" {
		return 7
	}
	return 6
}

// IsServiceDown diz se o cStat é a SEFAZ dizendo que está parada: o lote foi recusado, a
// nota com certeza não foi autorizada, e pode sair pela SVC com o mesmo número.
func IsServiceDown(cStat int) bool { return cStat == 108 || cStat == 109 }
