package entity

import "time"

// StatusCanceled: a nota foi cancelada por evento homologado.
const StatusCanceled Status = "canceled"

// EventStatus é o estado de um evento.
type EventStatus string

const (
	EventSigned     EventStatus = "signed"
	EventRegistered EventStatus = "registered"
	EventRejected   EventStatus = "rejected"
	EventUncertain  EventStatus = "uncertain"
	EventNotSent    EventStatus = "not_sent"
)

// Event é um evento da nota (cancelamento ou carta de correção).
type Event struct {
	ID        string
	TenantID  string
	InvoiceID string
	AccessKey string
	Type      string // 110111 cancelamento, 110110 carta de correção
	Seq       int
	Status    EventStatus
	CStat     int
	Motive    string
	Protocol  string
	Text      string
	SignedXML []byte
	ProcXML   []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Pending diz se o evento ainda pode ser enviado (o reenvio é seguro: a SEFAZ reconhece a
// duplicidade de evento e responde 573).
func (e *Event) Pending() bool {
	return e.Status == EventSigned || e.Status == EventUncertain || e.Status == EventNotSent
}

// EventDuplicate é o cStat da SEFAZ para evento já registrado.
const EventDuplicate = 573

// Cancel registra o cancelamento homologado.
func (i *Invoice) Cancel(at time.Time) { i.Status, i.UpdatedAt = StatusCanceled, at }
