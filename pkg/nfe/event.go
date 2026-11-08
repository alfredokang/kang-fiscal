package nfe

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Tipos de evento.
const (
	EventCancellation = "110111"
	EventCorrection   = "110110"
)

// CorrectionTerms é o xCondUso da carta de correção. A SEFAZ compara caractere a caractere
// com o texto do XSD (leiauteCCe_v1.00), que aceita esta forma sem acento e outra com.
const CorrectionTerms = "A Carta de Correcao e disciplinada pelo paragrafo 1o-A do art. 7o do Convenio S/N, de 15 de dezembro de 1970 e pode ser utilizada para regularizacao de erro ocorrido na emissao de documento fiscal, desde que o erro nao esteja relacionado com: I - as variaveis que determinam o valor do imposto tais como: base de calculo, aliquota, diferenca de preco, quantidade, valor da operacao ou da prestacao; II - a correcao de dados cadastrais que implique mudanca do remetente ou do destinatario; III - a data de emissao ou de saida."

var (
	// ErrJustification: a justificativa do cancelamento tem de 15 a 255 caracteres.
	ErrJustification = errors.New("nfe: justificativa do cancelamento com 15 a 255 caracteres")
	// ErrCorrection: a correção tem de 15 a 1000 caracteres.
	ErrCorrection = errors.New("nfe: texto da carta de correção com 15 a 1000 caracteres")
	// ErrSequence: a CC-e aceita até 20 correções por nota.
	ErrSequence = errors.New("nfe: sequência do evento de 1 a 20")
)

// Evento é um evento da NF-e (o elemento assinado é o infEvento).
type Evento struct {
	XMLName   xml.Name  `xml:"evento"`
	Xmlns     string    `xml:"xmlns,attr"`
	Versao    string    `xml:"versao,attr"`
	InfEvento InfEvento `xml:"infEvento"`
}

// InfEvento é o bloco assinado do evento.
type InfEvento struct {
	ID         string    `xml:"Id,attr"`
	COrgao     int       `xml:"cOrgao"`
	TpAmb      int       `xml:"tpAmb"`
	CNPJ       string    `xml:"CNPJ,omitempty"`
	CPF        string    `xml:"CPF,omitempty"`
	ChNFe      string    `xml:"chNFe"`
	DhEvento   string    `xml:"dhEvento"`
	TpEvento   string    `xml:"tpEvento"`
	NSeqEvento int       `xml:"nSeqEvento"`
	VerEvento  string    `xml:"verEvento"`
	DetEvento  DetEvento `xml:"detEvento"`
}

// DetEvento é o detalhe: cancelamento (nProt, xJust) ou CC-e (xCorrecao, xCondUso).
type DetEvento struct {
	Versao     string `xml:"versao,attr"`
	DescEvento string `xml:"descEvento"`
	NProt      string `xml:"nProt,omitempty"`
	XJust      string `xml:"xJust,omitempty"`
	XCorrecao  string `xml:"xCorrecao,omitempty"`
	XCondUso   string `xml:"xCondUso,omitempty"`
}

// EventParty é quem registra o evento sobre a nota.
type EventParty struct {
	OrgCode int // cOrgao: o cUF da autorizadora da nota
	TpAmb   int
	CNPJ    string
	Key     string // chave de acesso
	When    time.Time
}

func newEvent(p EventParty, tpEvento string, seq int, det DetEvento) *Evento {
	det.Versao = "1.00"
	return &Evento{Xmlns: Namespace, Versao: "1.00", InfEvento: InfEvento{
		ID:     fmt.Sprintf("ID%s%s%02d", tpEvento, p.Key, seq),
		COrgao: p.OrgCode, TpAmb: p.TpAmb, CNPJ: p.CNPJ, ChNFe: p.Key,
		DhEvento: p.When.Format("2006-01-02T15:04:05-07:00"), TpEvento: tpEvento, NSeqEvento: seq,
		VerEvento: "1.00", DetEvento: det,
	}}
}

// NewCancellation monta o cancelamento. O protocolo é o da autorização.
func NewCancellation(p EventParty, protocol, justification string) (*Evento, error) {
	j := cleanText(justification)
	if n := utf8.RuneCountInString(j); n < 15 || n > 255 {
		return nil, ErrJustification
	}
	return newEvent(p, EventCancellation, 1, DetEvento{DescEvento: "Cancelamento", NProt: protocol, XJust: j}), nil
}

// NewCorrection monta a carta de correção. Cada nova CC-e da mesma nota leva a sequência
// seguinte, e a última SUBSTITUI as anteriores: o texto precisa trazer todas as correções.
func NewCorrection(p EventParty, seq int, text string) (*Evento, error) {
	if seq < 1 || seq > 20 {
		return nil, ErrSequence
	}
	t := cleanText(text)
	if n := utf8.RuneCountInString(t); n < 15 || n > 1000 {
		return nil, ErrCorrection
	}
	return newEvent(p, EventCorrection, seq, DetEvento{DescEvento: "Carta de Correcao", XCorrecao: t, XCondUso: CorrectionTerms}), nil
}

// MarshalEvent serializa o evento sem declaração XML, pronto para a assinatura do infEvento.
func MarshalEvent(e *Evento) ([]byte, error) { return xml.Marshal(e) }

// EventBatch embrulha o evento assinado no envEvento.
func EventBatch(batchID int64, signedEvento []byte) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, `<envEvento xmlns="%s" versao="1.00"><idLote>%d</idLote>`, Namespace, batchID)
	b.Write(stripDeclaration(signedEvento))
	b.WriteString(`</envEvento>`)
	return b.Bytes()
}

// EventProc monta o procEventoNFe: o evento assinado com o retorno da SEFAZ. É o arquivo
// que prova o cancelamento ou a correção.
func EventProc(signedEvento, retEvento []byte) []byte {
	var b bytes.Buffer
	b.WriteString(`<procEventoNFe xmlns="` + Namespace + `" versao="1.00">`)
	b.Write(stripDeclaration(signedEvento))
	b.Write(stripDeclaration(retEvento))
	b.WriteString(`</procEventoNFe>`)
	return b.Bytes()
}

// cleanText junta espaços: a SEFAZ rejeita espaço duplo e nas pontas.
func cleanText(s string) string { return strings.Join(strings.Fields(s), " ") }
