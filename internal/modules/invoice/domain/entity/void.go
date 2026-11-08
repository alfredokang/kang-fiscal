package entity

import "time"

// Void é uma inutilização de faixa de números.
type Void struct {
	ID              string
	TenantID        string
	IssuerRef       string
	Environment     string
	Model, Series   int
	From, To        int
	Justification   string
	ReservationRefs []string
	Status          EventStatus // reaproveita signed/rejected/uncertain/not_sent; homologada = "voided"
	CStat           int
	Motive          string
	Protocol        string
	SignedXML       []byte
	ProcXML         []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// VoidDone é o estado da inutilização homologada.
const VoidDone EventStatus = "voided"
