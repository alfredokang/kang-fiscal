// Package service monta a NF-e a partir do pedido, do emitente e dos tributos calculados.
// É regra pura: não lê banco, não chama a SEFAZ.
package service

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain"
	"github.com/alfredokang/kang-fiscal/pkg/accesskey"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
)

// HomologationName é o que a SEFAZ exige no nome do destinatário em homologação
// (rejeição 598 sem ele).
const HomologationName = "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"

// Address é um endereço.
type Address struct {
	Street, Number, Complement, District string
	CityCode, CityName, UF, ZipCode      string
	Phone                                string
}

// IssuerSnapshot é o emitente congelado no momento da emissão (Constituição, Art. 2).
type IssuerSnapshot struct {
	CNPJ, Name, TradeName, IE string
	CRT                       int
	Address                   Address
	Environment               string // homologation | production
}

// Recipient é o destinatário.
type Recipient struct {
	CNPJ, CPF, Name, IE string
	// IEIndicator é o indIEDest: 1 contribuinte, 2 isento, 9 não contribuinte.
	IEIndicator int
	Email       string
	Address     *Address
}

// Line é um item pedido.
type Line struct {
	Code, Description, NCM, CEST, GTIN, Unit string
	Quantity, UnitPrice                      decimal.Fixed
	Discount, Freight, Insurance, Other      decimal.Fixed
	Origin                                   int
	Profile                                  string
}

// Payment é uma forma de pagamento.
type Payment struct {
	Method string // tPag: 01 dinheiro, 03 crédito, 04 débito, 15 boleto, 17 PIX, 90 sem pagamento
	Amount decimal.Fixed
}

// Draft é o pedido de emissão já validado no formato.
type Draft struct {
	Model, Series  int
	NatOp          string
	Issued         time.Time
	Purpose        int // finNFe: 1 normal
	Presence       int // indPres: 1 presencial, 2 internet, 9 outros
	FinalConsumer  bool
	Recipient      *Recipient
	Lines          []Line
	Payments       []Payment
	FreightMode    int // modFrete: 9 sem frete
	AdditionalInfo string
	RespTec        *nfe.InfRespTec
}

// Validate confere o que impede a emissão antes de reservar número.
func (d Draft) Validate(issuer IssuerSnapshot) error {
	if d.Model != 55 {
		// A NFC-e precisa do QR Code assinado com o CSC: entra na fase F2.
		return fmt.Errorf("%w: por enquanto só NF-e modelo 55", domain.ErrNotSupported)
	}
	if strings.TrimSpace(d.NatOp) == "" || len(d.Lines) == 0 || len(d.Payments) == 0 {
		return domain.ErrRequiredField
	}
	if len(d.Lines) > 990 {
		return fmt.Errorf("%w: no máximo 990 itens por nota", domain.ErrInvalidRequest)
	}
	if d.Recipient == nil {
		return fmt.Errorf("%w: a NF-e exige destinatário", domain.ErrRequiredField)
	}
	if d.Recipient.Address == nil {
		return fmt.Errorf("%w: endereço do destinatário", domain.ErrRequiredField)
	}
	if Interstate(d, issuer) && d.FinalConsumer && d.Recipient.IEIndicator == 9 {
		// Venda para consumidor final de outra UF exige a partilha do ICMS (DIFAL), que o
		// motor ainda não calcula. Emitir sem ela é rejeição; recusar aqui é honesto.
		return fmt.Errorf("%w: venda interestadual a consumidor final não contribuinte (DIFAL)", domain.ErrNotSupported)
	}
	for i, l := range d.Lines {
		if len(l.NCM) != 8 || strings.Trim(l.NCM, "0123456789") != "" {
			return fmt.Errorf("%w: item %d — NCM tem 8 dígitos", domain.ErrInvalidRequest, i+1)
		}
		if l.Quantity.Scale() > 4 || l.UnitPrice.Scale() > 10 {
			return fmt.Errorf("%w: item %d — quantidade até 4 casas, valor unitário até 10", domain.ErrInvalidRequest, i+1)
		}
		if strings.TrimSpace(l.Description) == "" || strings.TrimSpace(l.Code) == "" || strings.TrimSpace(l.Unit) == "" || l.Profile == "" {
			return fmt.Errorf("%w: item %d — código, descrição, unidade e perfil tributário", domain.ErrRequiredField, i+1)
		}
	}
	return nil
}

// Interstate diz se a operação é interestadual (idDest = 2).
func Interstate(d Draft, issuer IssuerSnapshot) bool {
	return d.Recipient != nil && d.Recipient.Address != nil &&
		!strings.EqualFold(d.Recipient.Address.UF, issuer.Address.UF)
}

// Build monta a nota. taxes[i] é o resultado do motor para d.Lines[i].
func Build(d Draft, issuer IssuerSnapshot, number, code int, taxes []tax.Result) (*nfe.NFe, accesskey.Key, error) {
	if len(taxes) != len(d.Lines) {
		return nil, "", fmt.Errorf("%w: tributos de %d itens para %d itens", domain.ErrInvalidRequest, len(taxes), len(d.Lines))
	}
	ufCode := ufCodes[strings.ToUpper(issuer.Address.UF)]
	tpAmb := 2
	if issuer.Environment == "production" {
		tpAmb = 1
	}
	issued := d.Issued.In(timezoneOf(issuer.Address.UF))
	key, err := accesskey.Build(accesskey.Parts{UFCode: ufCode, Issued: issued, Document: issuer.CNPJ,
		Model: accesskey.Model(d.Model), Series: d.Series, Number: number, EmissionType: 1, Code: code})
	if err != nil {
		return nil, "", err
	}

	items := make([]nfe.Det, len(d.Lines))
	for i, l := range d.Lines {
		gtin := l.GTIN
		if gtin == "" {
			gtin = "SEM GTIN"
		}
		p := nfe.Prod{CProd: clean(l.Code, 60), CEAN: gtin, XProd: clean(l.Description, 120), NCM: l.NCM, CEST: l.CEST,
			CFOP: taxes[i].CFOP, UCom: clean(l.Unit, 6), QCom: l.Quantity, VUnCom: l.UnitPrice, VProd: taxes[i].VProd,
			CEANTrib: gtin, UTrib: clean(l.Unit, 6), QTrib: l.Quantity, VUnTrib: l.UnitPrice, IndTot: 1}
		for _, pair := range []struct {
			dst **nfe.Dec
			v   decimal.Fixed
		}{{&p.VFrete, l.Freight}, {&p.VSeg, l.Insurance}, {&p.VDesc, l.Discount}, {&p.VOutro, l.Other}} {
			if !pair.v.IsZero() {
				m, _ := pair.v.Rescale(2, decimal.HalfUp)
				*pair.dst = &m
			}
		}
		items[i] = nfe.Det{NItem: i + 1, Prod: p, Imposto: taxes[i].Imposto}
	}
	total, err := nfe.ComputeTotals(items, false)
	if err != nil {
		return nil, "", err
	}

	paid := decimal.Fixed{}
	pag := nfe.Pag{}
	for _, p := range d.Payments {
		amount, _ := p.Amount.Rescale(2, decimal.HalfUp)
		pag.DetPag = append(pag.DetPag, nfe.DetPag{TPag: p.Method, VPag: amount})
		paid, _ = paid.Add(amount)
	}
	if paid.Cmp(total.ICMSTot.VNF) < 0 && !(len(d.Payments) == 1 && d.Payments[0].Method == "90") {
		return nil, "", fmt.Errorf("%w: pagamentos (%s) menores que o valor da nota (%s)", domain.ErrInvalidRequest, paid, total.ICMSTot.VNF)
	}
	if change, _ := paid.Sub(total.ICMSTot.VNF); change.Cmp(decimal.Fixed{}) > 0 {
		c, _ := change.Rescale(2, decimal.HalfUp)
		pag.VTroco = &c
	}

	idDest := 1
	if Interstate(d, issuer) {
		idDest = 2
	}
	indFinal := 0
	if d.FinalConsumer {
		indFinal = 1
	}
	k := key.String()
	n := &nfe.NFe{InfNFe: nfe.InfNFe{
		ID: "NFe" + k,
		Ide: nfe.Ide{CUF: ufCode, CNF: k[35:43], NatOp: clean(d.NatOp, 60), Mod: d.Model, Serie: d.Series, NNF: number,
			DhEmi: issued.Format("2006-01-02T15:04:05-07:00"), TpNF: 1, IdDest: idDest, CMunFG: issuer.Address.CityCode,
			TpImp: 1, TpEmis: 1, CDV: int(k[43] - '0'), TpAmb: tpAmb, FinNFe: orDefault(d.Purpose, 1), IndFinal: indFinal,
			IndPres: orDefault(d.Presence, 2), ProcEmi: 0, VerProc: "kang-fiscal"},
		Emit: nfe.Emit{CNPJ: issuer.CNPJ, XNome: clean(issuer.Name, 60), XFant: clean(issuer.TradeName, 60),
			EnderEmit: address(issuer.Address, true), IE: issuer.IE, CRT: issuer.CRT},
		Dest:       recipient(*d.Recipient, tpAmb),
		Det:        items,
		Total:      total,
		Transp:     nfe.Transp{ModFrete: orDefault(d.FreightMode, 9)},
		Pag:        pag,
		InfRespTec: d.RespTec,
	}}
	if info := clean(d.AdditionalInfo, 5000); info != "" {
		n.InfNFe.InfAdic = &nfe.InfAdic{InfCpl: info}
	}
	return n, key, nil
}

func recipient(r Recipient, tpAmb int) *nfe.Dest {
	name := clean(r.Name, 60)
	if tpAmb == 2 {
		name = HomologationName
	}
	d := &nfe.Dest{CNPJ: r.CNPJ, CPF: r.CPF, XNome: name, IndIEDest: r.IEIndicator, IE: r.IE, Email: clean(r.Email, 60)}
	if r.IEIndicator != 1 {
		d.IE = "" // IE só para contribuinte; isento e não contribuinte vão sem
	}
	if r.Address != nil {
		a := address(*r.Address, false)
		d.EnderDest = &a
	}
	return d
}

func address(a Address, withCountry bool) nfe.Endereco {
	e := nfe.Endereco{XLgr: clean(a.Street, 60), Nro: clean(a.Number, 60), XCpl: clean(a.Complement, 60),
		XBairro: clean(a.District, 60), CMun: a.CityCode, XMun: clean(a.CityName, 60), UF: strings.ToUpper(a.UF),
		CEP: a.ZipCode, Fone: a.Phone}
	if withCountry {
		e.CPais, e.XPais = "1058", "BRASIL"
	}
	return e
}

// clean junta espaços e corta no tamanho do campo: a SEFAZ rejeita espaço duplo e texto
// acima do tamanho do leiaute.
func clean(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	for utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:max]))
	}
	return s
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

var ufCodes = map[string]int{
	"RO": 11, "AC": 12, "AM": 13, "RR": 14, "PA": 15, "AP": 16, "TO": 17,
	"MA": 21, "PI": 22, "CE": 23, "RN": 24, "PB": 25, "PE": 26, "AL": 27, "SE": 28, "BA": 29,
	"MG": 31, "ES": 32, "RJ": 33, "SP": 35, "PR": 41, "SC": 42, "RS": 43,
	"MS": 50, "MT": 51, "GO": 52, "DF": 53,
}

// timezoneOf é o fuso da UF do emitente: o dhEmi vai com o fuso local, e a SEFAZ confere a
// data contra o relógio dela. Sem horário de verão desde 2019.
func timezoneOf(uf string) *time.Location {
	switch strings.ToUpper(uf) {
	case "AC":
		return time.FixedZone("-05", -5*3600)
	case "AM", "RR", "RO", "MT", "MS":
		return time.FixedZone("-04", -4*3600)
	}
	return time.FixedZone("-03", -3*3600)
}

// TimezoneForUFCode é o fuso pelo código IBGE da UF (o cUF da chave de acesso).
func TimezoneForUFCode(code int) *time.Location {
	for uf, c := range ufCodes {
		if c == code {
			return timezoneOf(uf)
		}
	}
	return timezoneOf("")
}
