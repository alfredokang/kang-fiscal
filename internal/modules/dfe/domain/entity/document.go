// Package entity contém os documentos recebidos e o cursor da distribuição.
package entity

import (
	"encoding/xml"
	"strings"
	"time"
)

// Kind é o tipo do documento recebido.
type Kind string

const (
	KindSummary      Kind = "summary"       // resNFe: o resumo, antes da manifestação
	KindFull         Kind = "full"          // procNFe: o XML completo
	KindEventSummary Kind = "event_summary" // resEvento
	KindEvent        Kind = "event"         // procEventoNFe
)

// KindOf classifica pelo schema que a SEFAZ informa no docZip.
func KindOf(schema string) Kind {
	switch {
	case strings.HasPrefix(schema, "resNFe"):
		return KindSummary
	case strings.HasPrefix(schema, "procNFe"):
		return KindFull
	case strings.HasPrefix(schema, "resEvento"):
		return KindEventSummary
	}
	return KindEvent
}

// Document é um documento recebido da distribuição.
type Document struct {
	ID          string
	TenantID    string
	IssuerRef   string // o emitente que é o DESTINATÁRIO desta nota
	NSU         int64
	Schema      string
	Kind        Kind
	AccessKey   string
	EmitterDoc  string
	EmitterName string
	Total       string
	IssuedAt    string
	Situation   string // cSitNFe: 1 autorizada, 2 denegada, 3 cancelada
	XML         []byte
	// Manifestation é a última manifestação registrada: "", 210200, 210210, 210220, 210240.
	Manifestation string
	CreatedAt     time.Time
}

// Describe lê do XML os campos que a lista mostra. O XML é guardado inteiro; isto é só o
// índice para quem procura "a nota de R$ 1.500 do fornecedor X".
func (d *Document) Describe() {
	var v struct {
		ChNFe   string `xml:"chNFe"`
		CNPJ    string `xml:"CNPJ"`
		CPF     string `xml:"CPF"`
		XNome   string `xml:"xNome"`
		VNF     string `xml:"vNF"`
		DhEmi   string `xml:"dhEmi"`
		CSitNFe string `xml:"cSitNFe"`
		InfNFe  *struct {
			ID  string `xml:"Id,attr"`
			Ide struct {
				DhEmi string `xml:"dhEmi"`
			} `xml:"ide"`
			Emit struct {
				CNPJ  string `xml:"CNPJ"`
				XNome string `xml:"xNome"`
			} `xml:"emit"`
			Total struct {
				VNF string `xml:"ICMSTot>vNF"`
			} `xml:"total"`
		} `xml:"NFe>infNFe"`
		EvChNFe string `xml:"evento>infEvento>chNFe"`
	}
	if err := xml.Unmarshal(d.XML, &v); err != nil {
		return
	}
	switch d.Kind {
	case KindSummary, KindEventSummary:
		d.AccessKey, d.EmitterDoc, d.EmitterName, d.Total, d.IssuedAt, d.Situation =
			v.ChNFe, v.CNPJ+v.CPF, v.XNome, v.VNF, v.DhEmi, v.CSitNFe
	case KindFull:
		if i := v.InfNFe; i != nil {
			d.AccessKey = strings.TrimPrefix(i.ID, "NFe")
			d.EmitterDoc, d.EmitterName, d.Total, d.IssuedAt = i.Emit.CNPJ, i.Emit.XNome, i.Total.VNF, i.Ide.DhEmi
		}
	default:
		d.AccessKey = v.EvChNFe
	}
}

// Cursor é a posição da distribuição de um emitente.
type Cursor struct {
	TenantID    string
	IssuerRef   string
	LastNSU     int64
	MaxNSU      int64
	NextQueryAt time.Time
	LastCStat   int
}

// Wait é a espera que a SEFAZ exige depois de "nenhum documento" (137) ou de "consumo
// indevido" (656). Consultar antes disso vira bloqueio de uma hora.
const Wait = time.Hour

// After atualiza o cursor com a resposta e diz se vale consultar de novo agora: só quando
// a SEFAZ entregou um lote e ainda há NSU por baixar.
func (c *Cursor) After(cStat int, last, max int64, now time.Time) (more bool) {
	c.LastCStat = cStat
	switch cStat {
	case 138:
		c.LastNSU, c.MaxNSU = last, max
		if last < max {
			return true
		}
		c.NextQueryAt = now.Add(Wait)
	case 137:
		if last > 0 {
			c.LastNSU = last
		}
		c.MaxNSU = max
		c.NextQueryAt = now.Add(Wait)
	default: // 656 e qualquer rejeição: espera
		c.NextQueryAt = now.Add(Wait)
	}
	return false
}
