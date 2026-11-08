// Package nfse emite a NFS-e no padrão nacional: monta a DPS (Declaração de Prestação de
// Serviço), que é assinada e enviada ao Sistema Nacional (Sefin), e conversa com a API REST
// dele.
//
// Diferenças que importam em relação à NF-e:
//   - A DPS é do prestador; a NFS-e (com o número nNFSe e a chave de 50 posições) é gerada
//     pelo Sistema Nacional e volta na resposta.
//   - IBS e CBS: a DPS só classifica (CST e cClassTrib); quem calcula é o Sistema Nacional.
//   - A API é REST/JSON, com o XML assinado em gzip+base64.
//
// O leiaute segue o pacote de schemas da NT 009 (20/10/2026) — testdata/schemas.
package nfse

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/decimal"
)

// Namespace é o namespace da DPS e da NFS-e.
const Namespace = "http://www.sped.fazenda.gov.br/nfse"

// Dec é o número do leiaute.
type Dec = decimal.Fixed

// ErrInvalid: campo fora do leiaute.
var ErrInvalid = errors.New("nfse: DPS inválida")

// DPS é a declaração assinada (o elemento assinado é o infDPS).
type DPS struct {
	XMLName xml.Name `xml:"DPS"`
	Xmlns   string   `xml:"xmlns,attr"`
	Versao  string   `xml:"versao,attr"`
	InfDPS  InfDPS   `xml:"infDPS"`
}

// InfDPS é o bloco assinado.
type InfDPS struct {
	ID       string  `xml:"Id,attr"`
	TpAmb    int     `xml:"tpAmb"`
	DhEmi    string  `xml:"dhEmi"`
	VerAplic string  `xml:"verAplic"`
	Serie    string  `xml:"serie"`
	NDPS     string  `xml:"nDPS"`
	DCompet  string  `xml:"dCompet"` // AAAA-MM-DD da competência
	TpEmit   int     `xml:"tpEmit"`  // 1 prestador
	FinNFSe  int     `xml:"finNFSe"` // 0 regular (NT 009)
	CLocEmi  string  `xml:"cLocEmi"` // município do emitente (IBGE)
	Prest    Prest   `xml:"prest"`
	Toma     *Pessoa `xml:"toma,omitempty"`
	Serv     Serv    `xml:"serv"`
	Valores  Valores `xml:"valores"`
	IBSCBS   *IBSCBS `xml:"IBSCBS,omitempty"`
}

// Prest é o prestador. Quando ele é o emitente (tpEmit 1), nome e endereço vêm do
// cadastro do Sistema Nacional e não são informados.
type Prest struct {
	CNPJ    string  `xml:"CNPJ,omitempty"`
	CPF     string  `xml:"CPF,omitempty"`
	IM      string  `xml:"IM,omitempty"`
	RegTrib RegTrib `xml:"regTrib"`
}

// RegTrib é o regime tributário do prestador.
type RegTrib struct {
	OpSimpNac   int  `xml:"opSimpNac"` // 1 não optante, 2 MEI, 3 ME/EPP
	RegApTribSN *int `xml:"regApTribSN,omitempty"`
	RegEspTrib  int  `xml:"regEspTrib"` // 0 nenhum
}

// Pessoa é o tomador.
type Pessoa struct {
	CNPJ  string    `xml:"CNPJ,omitempty"`
	CPF   string    `xml:"CPF,omitempty"`
	IM    string    `xml:"IM,omitempty"`
	XNome string    `xml:"xNome"`
	End   *Endereco `xml:"end,omitempty"`
	Fone  string    `xml:"fone,omitempty"`
	Email string    `xml:"email,omitempty"`
}

// Endereco é o endereço nacional.
type Endereco struct {
	EndNac  EndNac `xml:"endNac"`
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl,omitempty"`
	XBairro string `xml:"xBairro"`
}

// EndNac é o município e o CEP.
type EndNac struct {
	CMun string `xml:"cMun"`
	CEP  string `xml:"CEP"`
}

// Serv é o serviço.
type Serv struct {
	LocPrest LocPrest `xml:"locPrest"`
	CServ    CServ    `xml:"cServ"`
}

// LocPrest é onde o serviço foi prestado.
type LocPrest struct {
	CLocPrestacao string `xml:"cLocPrestacao"`
}

// CServ é a classificação do serviço.
type CServ struct {
	CTribNac  string `xml:"cTribNac"` // item/subitem da LC 116 + desdobro, 6 dígitos
	CTribMun  string `xml:"cTribMun,omitempty"`
	CNBS      string `xml:"cNBS,omitempty"`
	XDescServ string `xml:"xDescServ"`
}

// Valores são os valores e os tributos.
type Valores struct {
	VServPrest      VServPrest       `xml:"vServPrest"`
	VDescCondIncond *VDescCondIncond `xml:"vDescCondIncond,omitempty"`
	Trib            Trib             `xml:"trib"`
}

// VServPrest é o valor do serviço.
type VServPrest struct {
	VServ Dec `xml:"vServ"`
}

// VDescCondIncond são os descontos.
type VDescCondIncond struct {
	VDescIncond *Dec `xml:"vDescIncond,omitempty"`
	VDescCond   *Dec `xml:"vDescCond,omitempty"`
}

// Trib são os tributos da DPS.
type Trib struct {
	TribMun TribMun  `xml:"tribMun"`
	TribFed *TribFed `xml:"tribFed,omitempty"`
	TotTrib TotTrib  `xml:"totTrib"`
}

// TribMun é o ISSQN.
type TribMun struct {
	TribISSQN  int  `xml:"tribISSQN"`  // 1 tributável, 2 imunidade, 3 exportação, 4 não incidência
	TpRetISSQN int  `xml:"tpRetISSQN"` // 1 não retido, 2 retido pelo tomador, 3 pelo intermediário
	PAliq      *Dec `xml:"pAliq,omitempty"`
}

// TribFed é o PIS/COFINS e as retenções federais.
type TribFed struct {
	PisCofins *PisCofins `xml:"piscofins,omitempty"`
}

// PisCofins é o PIS e a COFINS.
type PisCofins struct {
	CST          string `xml:"CST"`
	VBCPisCofins *Dec   `xml:"vBCPisCofins,omitempty"`
	PAliqPis     *Dec   `xml:"pAliqPis,omitempty"`
	PAliqCofins  *Dec   `xml:"pAliqCofins,omitempty"`
	VPis         *Dec   `xml:"vPis,omitempty"`
	VCofins      *Dec   `xml:"vCofins,omitempty"`
}

// TotTrib é a Lei da Transparência: informe exatamente um.
type TotTrib struct {
	// PTotTribSN é o percentual do Simples Nacional.
	PTotTribSN *Dec `xml:"pTotTribSN,omitempty"`
	// IndTotTrib 0 = não informar estimativa (Decreto 8.264/2014).
	IndTotTrib *int `xml:"indTotTrib,omitempty"`
}

// IBSCBS é a classificação do IBS e da CBS. Os valores são calculados pelo Sistema Nacional.
type IBSCBS struct {
	IndFinal *int          `xml:"indFinal,omitempty"` // 1 uso ou consumo pessoal
	CIndOp   string        `xml:"cIndOp,omitempty"`   // código indicador da operação
	IndDest  int           `xml:"indDest"`            // 0 o destinatário é o tomador
	Valores  IBSCBSValores `xml:"valores"`
}

// IBSCBSValores envolve a classificação.
type IBSCBSValores struct {
	Trib IBSCBSTrib `xml:"trib"`
}

// IBSCBSTrib é o CST e o cClassTrib. O grupo gIBSCBS (crédito presumido, diferimento) é
// opcional e fica para quando houver caso real.
type IBSCBSTrib struct {
	CST        string `xml:"CST"`
	CClassTrib string `xml:"cClassTrib"`
}

// ID monta o Id da DPS: "DPS" + município (7) + tipo de inscrição (1 CPF, 2 CNPJ) +
// inscrição (14) + série (5) + número (15).
func ID(cityCode, document string, series, number int) (string, error) {
	tp := "2"
	doc := strings.ToUpper(document)
	if len(doc) == 11 {
		tp, doc = "1", "000"+doc
	}
	if len(cityCode) != 7 || len(doc) != 14 || series < 0 || series > 89999 || number < 1 {
		return "", fmt.Errorf("%w: Id (município %q, documento %q, série %d, número %d)", ErrInvalid, cityCode, document, series, number)
	}
	return fmt.Sprintf("DPS%s%s%s%05d%015d", cityCode, tp, doc, series, number), nil
}

// Marshal serializa a DPS sem declaração XML, pronta para assinar o infDPS.
func Marshal(d *DPS) ([]byte, error) {
	d.Xmlns = Namespace
	if d.Versao == "" {
		d.Versao = "1.01"
	}
	return xml.Marshal(d)
}

// Timestamp formata o dhEmi. O Sistema Nacional recusa dhEmi no futuro: o horário é o de
// agora, com o fuso do prestador.
func Timestamp(t time.Time) string { return t.Format("2006-01-02T15:04:05-07:00") }
