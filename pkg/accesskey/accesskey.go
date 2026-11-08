// Package accesskey monta e confere a chave de acesso de 44 posições da NF-e e da NFC-e.
//
//	cUF(2) AAMM(4) CNPJ/CPF(14) mod(2) serie(3) nNF(9) tpEmis(1) cNF(8) cDV(1)
//
// A chave é gerada ANTES do envio e gravada com a nota (Constituição, Art. 6.1): é por ela
// que se consulta a SEFAZ depois de um timeout, em vez de reenviar às cegas.
package accesskey

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Length é o tamanho da chave.
const Length = 44

var (
	// ErrInvalid: tamanho, caractere ou dígito verificador errado.
	ErrInvalid = errors.New("chave de acesso inválida")
	// ErrField: algum campo da chave fora da faixa do leiaute.
	ErrField = errors.New("campo da chave de acesso fora da faixa")
)

// Model é o modelo do documento.
type Model int

const (
	// NFe é a NF-e, modelo 55.
	NFe Model = 55
	// NFCe é a NFC-e, modelo 65.
	NFCe Model = 65
)

// Parts são os campos que formam a chave.
type Parts struct {
	UFCode   int       // código IBGE da UF do emitente (35 = SP, 31 = MG)
	Issued   time.Time // só ano e mês entram na chave, no fuso do emitente
	Document string    // CNPJ (14) ou CPF do emitente; CPF entra com zeros à esquerda
	Model    Model
	Series   int // 0 a 999
	Number   int // 1 a 999.999.999
	// EmissionType é o tpEmis: 1 normal, 6 SVC-AN, 7 SVC-RS, 9 offline da NFC-e...
	EmissionType int
	// Code é o cNF: 8 dígitos aleatórios. Use NewCode.
	Code int
}

// Key é uma chave de acesso válida.
type Key string

// Build monta a chave a partir dos campos e calcula o dígito.
func Build(p Parts) (Key, error) {
	if p.UFCode < 11 || p.UFCode > 53 || p.Model != NFe && p.Model != NFCe ||
		p.Series < 0 || p.Series > 999 || p.Number < 1 || p.Number > 999_999_999 ||
		p.EmissionType < 1 || p.EmissionType > 9 || p.Code < 0 || p.Code > 99_999_999 {
		return "", ErrField
	}
	doc := strings.ToUpper(p.Document)
	if len(doc) == 11 {
		doc = "000" + doc
	}
	if len(doc) != 14 {
		return "", fmt.Errorf("%w: documento do emitente com %d posições", ErrField, len(doc))
	}
	base := fmt.Sprintf("%02d%02d%02d%s%02d%03d%09d%d%08d",
		p.UFCode, p.Issued.Year()%100, int(p.Issued.Month()), doc,
		int(p.Model), p.Series, p.Number, p.EmissionType, p.Code)
	return Key(base + string(checkDigit(base))), nil
}

// Parse confere uma chave recebida de fora.
func Parse(raw string) (Key, error) {
	s := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), " ", ""))
	s = strings.TrimPrefix(s, "NFE")
	if len(s) != Length {
		return "", ErrInvalid
	}
	for i := 0; i < Length; i++ {
		c := s[i]
		isDigit := c >= '0' && c <= '9'
		// Só as posições do CNPJ (7 a 20) aceitam letra: é o CNPJ alfanumérico.
		inDocument := i >= 6 && i < 20
		if !isDigit && !(inDocument && c >= 'A' && c <= 'Z') {
			return "", ErrInvalid
		}
	}
	if checkDigit(s[:43]) != s[43] {
		return "", ErrInvalid
	}
	return Key(s), nil
}

// NewCode sorteia o cNF.
//
// Aleatório de verdade (crypto/rand): o cNF existe para que ninguém adivinhe a chave de
// uma nota a partir do número dela e consulte o documento de outra empresa. Por isso
// também é proibido ser igual ao número da nota — a SEFAZ rejeita.
func NewCode(number int) (int, error) {
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
		if err != nil {
			return 0, err
		}
		code := int(n.Int64())
		if code != number%100_000_000 {
			return code, nil
		}
	}
}

// checkDigit é o módulo 11 com pesos 2 a 9 da direita para a esquerda. Letra do CNPJ
// alfanumérico vale o código ASCII menos 48, como no dígito do próprio CNPJ.
func checkDigit(s string) byte {
	sum, w := 0, 2
	for i := len(s) - 1; i >= 0; i-- {
		sum += int(s[i]-'0') * w
		w++
		if w > 9 {
			w = 2
		}
	}
	r := sum % 11
	if r < 2 {
		return '0'
	}
	return byte('0' + 11 - r)
}

// UFCode é o código IBGE da UF do emitente.
func (k Key) UFCode() string { return string(k)[0:2] }

// Document é o CNPJ (ou CPF com zeros à esquerda) do emitente.
func (k Key) Document() string { return string(k)[6:20] }

// Model é o modelo do documento.
func (k Key) Model() Model {
	if string(k)[20:22] == "65" {
		return NFCe
	}
	return NFe
}

// EmissionType é o tpEmis.
func (k Key) EmissionType() int { return int(string(k)[34] - '0') }

// String devolve a chave crua.
func (k Key) String() string { return string(k) }
