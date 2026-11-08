package entity

import "time"

// EventStatus é o estado de um pedido de evento.
type EventStatus string

const (
	EventSigned     EventStatus = "signed"     // assinado e gravado, antes do envio
	EventRegistered EventStatus = "registered" // registrado no Sistema Nacional
	EventRejected   EventStatus = "rejected"   // recusado (E-code)
	EventUncertain  EventStatus = "uncertain"  // sem resposta: conferir na Sefin antes de reenviar
	EventNotSent    EventStatus = "not_sent"   // certificado recusado: nada foi processado
)

// Tipos de evento que o Kang Fiscal pede.
const (
	KindCancel   = "101101"
	KindAnalysis = "101103"
)

// Event é um pedido de evento da NFS-e.
type Event struct {
	ID, TenantID, DocumentID string
	Kind                     string
	Reason                   int
	Justification            string
	Status                   EventStatus
	ErrorCode, Message       string
	SignedXML, EventXML      []byte
	CreatedAt, UpdatedAt     time.Time
}

// Settle registra o resultado de um envio.
func (e *Event) Settle(status EventStatus, code, message string, at time.Time) {
	e.Status, e.ErrorCode, e.Message, e.UpdatedAt = status, code, message, at
}
