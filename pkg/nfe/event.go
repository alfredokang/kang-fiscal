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
	// EventSubstitution é o cancelamento por substituição da NFC-e: cancela a nota online
	// que ficou sem resposta e depois saiu autorizada, quando a venda já tinha sido coberta
	// por uma NFC-e em contingência offline.
	EventSubstitution = "110112"
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

// substDet é o detEvento do 110112, que tem outra ordem de campos.
type substDet struct {
	Versao      string `xml:"versao,attr"`
	DescEvento  string `xml:"descEvento"`
	COrgaoAutor int    `xml:"cOrgaoAutor"`
	TpAutor     int    `xml:"tpAutor"`
	VerAplic    string `xml:"verAplic"`
	NProt       string `xml:"nProt"`
	XJust       string `xml:"xJust"`
	ChNFeRef    string `xml:"chNFeRef"`
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

// SubstitutionEvent é o evento de cancelamento por substituição (110112).
type SubstitutionEvent struct {
	XMLName   xml.Name `xml:"evento"`
	Xmlns     string   `xml:"xmlns,attr"`
	Versao    string   `xml:"versao,attr"`
	InfEvento struct {
		ID         string   `xml:"Id,attr"`
		COrgao     int      `xml:"cOrgao"`
		TpAmb      int      `xml:"tpAmb"`
		CNPJ       string   `xml:"CNPJ,omitempty"`
		ChNFe      string   `xml:"chNFe"`
		DhEvento   string   `xml:"dhEvento"`
		TpEvento   string   `xml:"tpEvento"`
		NSeqEvento int      `xml:"nSeqEvento"`
		VerEvento  string   `xml:"verEvento"`
		DetEvento  substDet `xml:"detEvento"`
	} `xml:"infEvento"`
}

// NewSubstitution monta o cancelamento por substituição: a nota de `p.Key` (com protocolo
// `protocol`) é cancelada porque a venda foi coberta pela NFC-e `replacedBy`.
func NewSubstitution(p EventParty, protocol, justification, replacedBy, verAplic string) (*SubstitutionEvent, error) {
	j := cleanText(justification)
	if n := utf8.RuneCountInString(j); n < 15 || n > 255 {
		return nil, ErrJustification
	}
	e := &SubstitutionEvent{Xmlns: Namespace, Versao: "1.00"}
	i := &e.InfEvento
	i.ID = fmt.Sprintf("ID%s%s%02d", EventSubstitution, p.Key, 1)
	i.COrgao, i.TpAmb, i.CNPJ, i.ChNFe = p.OrgCode, p.TpAmb, p.CNPJ, p.Key
	i.DhEvento = p.When.Format("2006-01-02T15:04:05-07:00")
	i.TpEvento, i.NSeqEvento, i.VerEvento = EventSubstitution, 1, "1.00"
	i.DetEvento = substDet{Versao: "1.00", DescEvento: "Cancelamento por substituicao", COrgaoAutor: p.OrgCode,
		TpAutor: 1, VerAplic: verAplic, NProt: protocol, XJust: j, ChNFeRef: replacedBy}
	return e, nil
}

// MarshalSubstitution serializa o evento, pronto para assinar o infEvento.
func MarshalSubstitution(e *SubstitutionEvent) ([]byte, error) { return xml.Marshal(e) }

// Manifestações do destinatário (enviadas ao Ambiente Nacional, cOrgao 91).
const (
	ManifestConfirmation = "210200" // Confirmação da Operação: a mercadoria chegou
	ManifestAwareness    = "210210" // Ciência da Operação: libera o download do XML completo
	ManifestUnknown      = "210220" // Desconhecimento: a empresa não reconhece a nota
	ManifestNotPerformed = "210240" // Operação não Realizada (exige justificativa)
)

// OrgAN é o cOrgao do Ambiente Nacional.
const OrgAN = 91

var manifestDesc = map[string]string{
	ManifestConfirmation: "Confirmacao da Operacao",
	ManifestAwareness:    "Ciencia da Operacao",
	ManifestUnknown:      "Desconhecimento da Operacao",
	ManifestNotPerformed: "Operacao nao Realizada",
}

// ErrManifestType: tipo de manifestação desconhecido.
var ErrManifestType = errors.New("nfe: manifestação inválida (210200, 210210, 210220 ou 210240)")

// NewManifestation monta a manifestação do destinatário sobre a nota p.Key. p.CNPJ é o do
// DESTINATÁRIO (quem manifesta) e p.OrgCode é o do Ambiente Nacional (91).
func NewManifestation(p EventParty, tpEvento, justification string) (*Evento, error) {
	desc, ok := manifestDesc[tpEvento]
	if !ok {
		return nil, ErrManifestType
	}
	det := DetEvento{DescEvento: desc}
	if tpEvento == ManifestNotPerformed {
		j := cleanText(justification)
		if n := utf8.RuneCountInString(j); n < 15 || n > 255 {
			return nil, ErrJustification
		}
		det.XJust = j
	}
	p.OrgCode = OrgAN
	return newEvent(p, tpEvento, 1, det), nil
}
