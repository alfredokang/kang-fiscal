// Package entity contém o agregado da nota.
package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Status é o estado da nota.
//
//	signed ──► authorized          (cStat 100/150)
//	   │  ├──► denied              (110, 301, 302, 303: número consumido, sem validade)
//	   │  ├──► rejected            (qualquer outra rejeição: corrigir e emitir de novo)
//	   │  ├──► uncertain           (sem resposta conclusiva: consultar pela chave)
//	   │  └──► not_sent            (certificado recusado na conexão: nada foi processado)
//	uncertain ──► authorized | denied | rejected   (depois da consulta)
type Status string

const (
	StatusSigned     Status = "signed"
	StatusAuthorized Status = "authorized"
	StatusDenied     Status = "denied"
	StatusRejected   Status = "rejected"
	StatusUncertain  Status = "uncertain"
	StatusNotSent    Status = "not_sent"
)

// Final diz se o estado não muda mais por envio.
func (s Status) Final() bool { return s == StatusAuthorized || s == StatusDenied }

// Invoice é a nota emitida (ou em emissão).
type Invoice struct {
	ID             string
	TenantID       string
	IssuerRef      string
	IdempotencyKey string
	RequestHash    string
	Model          int
	Series         int
	Number         int
	ReservationRef string
	Environment    string
	AccessKey      string
	Status         Status
	CStat          int
	Motive         string
	Protocol       string
	TotalAmount    string // vNF, como texto do leiaute
	SignedXML      []byte
	AuthorizedXML  []byte // nfeProc; imutável depois de gravado
	XMLHash        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	AuthorizedAt   *time.Time
}

// HashRequest é a impressão digital do pedido, para a idempotência: a mesma chave com outro
// conteúdo é erro, não "a mesma nota".
func HashRequest(canonicalJSON []byte) string {
	sum := sha256.Sum256(canonicalJSON)
	return hex.EncodeToString(sum[:])
}

// Authorize registra a autorização e o nfeProc. O hash do XML fica com a nota: é o que
// prova, daqui a cinco anos, que o arquivo guardado é o que a SEFAZ autorizou.
func (i *Invoice) Authorize(cStat int, motive, protocol string, proc []byte, at time.Time) {
	i.Status, i.CStat, i.Motive, i.Protocol = StatusAuthorized, cStat, motive, protocol
	i.AuthorizedXML = proc
	sum := sha256.Sum256(proc)
	i.XMLHash = hex.EncodeToString(sum[:])
	i.AuthorizedAt, i.UpdatedAt = &at, at
}

// Settle registra um resultado que não é autorização.
func (i *Invoice) Settle(status Status, cStat int, motive string, at time.Time) {
	i.Status, i.CStat, i.Motive, i.UpdatedAt = status, cStat, motive, at
}

// IsDenial diz se o cStat é denegação: o número é consumido e a nota não vale.
func IsDenial(cStat int) bool {
	return cStat == 110 || cStat == 301 || cStat == 302 || cStat == 303
}

// IsAuthorization diz se o cStat autoriza o uso (150 é autorizada fora do prazo).
func IsAuthorization(cStat int) bool { return cStat == 100 || cStat == 150 }
