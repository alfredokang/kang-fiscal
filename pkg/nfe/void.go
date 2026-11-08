package nfe

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"unicode/utf8"
)

// ErrVoidRange: faixa inválida para inutilização.
var ErrVoidRange = errors.New("nfe: faixa de inutilização inválida (1 a 999.999.999, início ≤ fim)")

// InutNFe é o pedido de inutilização de uma faixa de números (o assinado é o infInut).
type InutNFe struct {
	XMLName xml.Name `xml:"inutNFe"`
	Xmlns   string   `xml:"xmlns,attr"`
	Versao  string   `xml:"versao,attr"`
	InfInut InfInut  `xml:"infInut"`
}

// InfInut é o bloco assinado.
type InfInut struct {
	ID     string `xml:"Id,attr"`
	TpAmb  int    `xml:"tpAmb"`
	XServ  string `xml:"xServ"`
	CUF    int    `xml:"cUF"`
	Ano    string `xml:"ano"`
	CNPJ   string `xml:"CNPJ"`
	Mod    int    `xml:"mod"`
	Serie  int    `xml:"serie"`
	NNFIni int    `xml:"nNFIni"`
	NNFFin int    `xml:"nNFFin"`
	XJust  string `xml:"xJust"`
}

// VoidRange é a faixa a inutilizar.
type VoidRange struct {
	TpAmb, CUF, Model, Series int
	Year                      int // ano da inutilização, com 4 dígitos
	CNPJ                      string
	From, To                  int
	Justification             string
}

// NewVoidRange monta o pedido de inutilização.
func NewVoidRange(v VoidRange) (*InutNFe, error) {
	if v.From < 1 || v.To < v.From || v.To > 999_999_999 {
		return nil, ErrVoidRange
	}
	j := cleanText(v.Justification)
	if n := utf8.RuneCountInString(j); n < 15 || n > 255 {
		return nil, ErrJustification
	}
	year := fmt.Sprintf("%02d", v.Year%100)
	return &InutNFe{Xmlns: Namespace, Versao: "4.00", InfInut: InfInut{
		ID:    fmt.Sprintf("ID%02d%s%s%02d%03d%09d%09d", v.CUF, year, v.CNPJ, v.Model, v.Series, v.From, v.To),
		TpAmb: v.TpAmb, XServ: "INUTILIZAR", CUF: v.CUF, Ano: year, CNPJ: v.CNPJ, Mod: v.Model,
		Serie: v.Series, NNFIni: v.From, NNFFin: v.To, XJust: j,
	}}, nil
}

// MarshalVoid serializa o pedido, pronto para assinar o infInut.
func MarshalVoid(i *InutNFe) ([]byte, error) { return xml.Marshal(i) }

// VoidProc monta o procInutNFe: o pedido assinado e o retorno homologado.
func VoidProc(signed, ret []byte) []byte {
	var b bytes.Buffer
	b.WriteString(`<procInutNFe xmlns="` + Namespace + `" versao="4.00">`)
	b.Write(stripDeclaration(signed))
	b.Write(stripDeclaration(ret))
	b.WriteString(`</procInutNFe>`)
	return b.Bytes()
}
