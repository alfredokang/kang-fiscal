// Package entity contém a NFS-e emitida pelo Kang Fiscal.
package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Status é o estado da NFS-e.
type Status string

const (
	StatusSigned     Status = "signed"     // DPS assinada e gravada, antes do envio
	StatusAuthorized Status = "authorized" // NFS-e gerada pelo Sistema Nacional
	StatusRejected   Status = "rejected"   // DPS recusada (E-code)
	StatusUncertain  Status = "uncertain"  // sem resposta: consultar a DPS antes de reenviar
	StatusNotSent    Status = "not_sent"   // certificado recusado: nada foi processado
	StatusCanceled   Status = "canceled"   // cancelamento registrado no Sistema Nacional
)

// Andamento do cancelamento por análise fiscal (e101103).
const (
	CancelAnalysis = "analysis" // pedido feito; a nota segue válida até o município decidir
	CancelDenied   = "denied"   // o município negou; a nota segue válida
)

// ServiceInvoice é a NFS-e (e a DPS que a originou).
type ServiceInvoice struct {
	ID, TenantID, IssuerRef string
	IdempotencyKey          string
	RequestHash             string
	Series, Number          int // série e número da DPS
	ReservationRef          string
	Environment             string
	DPSID                   string // Id da DPS: é por ele que se consulta depois de um timeout
	AccessKey               string // chave de 50 posições, quando gerada
	NFSeNumber              string
	Status                  Status
	CancelState             string
	ErrorCode, Message      string
	Total                   string
	SignedXML, NFSeXML      []byte
	XMLHash                 string
	CreatedAt, UpdatedAt    time.Time
}

// Authorize registra a NFS-e gerada.
func (s *ServiceInvoice) Authorize(key, number string, doc []byte, at time.Time) {
	sum := sha256.Sum256(doc)
	s.Status, s.AccessKey, s.NFSeNumber, s.NFSeXML, s.XMLHash = StatusAuthorized, key, number, doc, hex.EncodeToString(sum[:])
	s.ErrorCode, s.Message, s.UpdatedAt = "", "", at
}

// Settle registra um resultado que não é autorização.
func (s *ServiceInvoice) Settle(status Status, code, message string, at time.Time) {
	s.Status, s.ErrorCode, s.Message, s.UpdatedAt = status, code, message, at
}

// Cancel marca a nota como cancelada.
func (s *ServiceInvoice) Cancel(at time.Time) {
	s.Status, s.CancelState, s.UpdatedAt = StatusCanceled, "", at
}
