// Package valueobject valida no construtor tudo o que forma o cadastro do emitente.
package valueobject

import (
	"strings"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
	"github.com/alfredokang/kang-fiscal/pkg/document"
	"github.com/google/uuid"
)

// IssuerID é o UUID v7 do emitente, gerado no Go (Constituição, Art. 4.2).
type IssuerID string

// NewIssuerID gera um id novo.
func NewIssuerID() (IssuerID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return IssuerID(id.String()), nil
}

// ParseIssuerID confere um id recebido de fora.
func ParseIssuerID(raw string) (IssuerID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", domain.ErrInvalidIssuerID
	}
	return IssuerID(id.String()), nil
}

// ParseCNPJ aceita o CNPJ numérico ou alfanumérico, com ou sem pontuação.
func ParseCNPJ(raw string) (document.CNPJ, error) {
	c, err := document.ParseCNPJ(raw)
	if err != nil {
		return "", domain.ErrInvalidCNPJ
	}
	return c, nil
}

// UF é a sigla da unidade da federação.
type UF string

// ufCodes é o código IBGE de cada UF: é ele, e não a sigla, que vai na chave de acesso e
// no cUF do XML.
var ufCodes = map[UF]int{
	"RO": 11, "AC": 12, "AM": 13, "RR": 14, "PA": 15, "AP": 16, "TO": 17,
	"MA": 21, "PI": 22, "CE": 23, "RN": 24, "PB": 25, "PE": 26, "AL": 27, "SE": 28, "BA": 29,
	"MG": 31, "ES": 32, "RJ": 33, "SP": 35,
	"PR": 41, "SC": 42, "RS": 43,
	"MS": 50, "MT": 51, "GO": 52, "DF": 53,
}

// ParseUF aceita a sigla em qualquer caixa.
func ParseUF(raw string) (UF, error) {
	uf := UF(strings.ToUpper(strings.TrimSpace(raw)))
	if _, ok := ufCodes[uf]; !ok {
		return "", domain.ErrInvalidUF
	}
	return uf, nil
}

// Code é o código IBGE da UF.
func (u UF) Code() int { return ufCodes[u] }

// CityCode é o código IBGE do município, com 7 dígitos. A nota não aceita o nome da cidade.
type CityCode string

// ParseCityCode confere o formato e que o município é da UF: os dois primeiros dígitos do
// código IBGE do município são o código da UF. Endereço com cidade de um estado e UF de
// outro é rejeição certa na SEFAZ, e é o erro que o cadastro manual mais produz.
func ParseCityCode(raw string, uf UF) (CityCode, error) {
	s := strings.TrimSpace(raw)
	if len(s) != 7 || !onlyDigits(s) {
		return "", domain.ErrInvalidCityCode
	}
	if s[:2] != itoa2(uf.Code()) {
		return "", domain.ErrCityOutsideUF
	}
	return CityCode(s), nil
}

// ParseZipCode aceita o CEP com ou sem hífen.
func ParseZipCode(raw string) (string, error) {
	s := strings.NewReplacer("-", "", ".", "", " ", "").Replace(raw)
	if len(s) != 8 || !onlyDigits(s) {
		return "", domain.ErrInvalidZipCode
	}
	return s, nil
}

// TaxRegime é o CRT do XML.
//
// Ele decide como o ICMS de cada item sai: no Simples (1, 2 e 4) por CSOSN, no regime
// normal (3) por CST. E, na reforma tributária, decide desde quando o IBS e a CBS são
// obrigatórios no documento — Simples e MEI só a partir de 2027.
type TaxRegime int

const (
	SimplesNacional       TaxRegime = 1
	SimplesExcessoReceita TaxRegime = 2
	RegimeNormal          TaxRegime = 3
	MEI                   TaxRegime = 4
)

// ParseTaxRegime confere o CRT.
func ParseTaxRegime(v int) (TaxRegime, error) {
	switch TaxRegime(v) {
	case SimplesNacional, SimplesExcessoReceita, RegimeNormal, MEI:
		return TaxRegime(v), nil
	}
	return 0, domain.ErrInvalidTaxRegime
}

// IsSimples diz se o ICMS sai por CSOSN.
func (t TaxRegime) IsSimples() bool { return t != RegimeNormal }

// StateRegistration é a inscrição estadual: só dígitos, ou "ISENTO". Por isso é texto.
//
// O dígito da IE muda de algoritmo em cada UF; a conferência por UF entra com a tabela de
// regras, sem travar o cadastro até lá.
type StateRegistration string

// ParseStateRegistration normaliza a pontuação e aceita vazio (cadastro em andamento).
func ParseStateRegistration(raw string) (StateRegistration, error) {
	s := strings.ToUpper(strings.NewReplacer(".", "", "-", "", "/", "", " ", "").Replace(raw))
	if s == "" || s == "ISENTO" {
		return StateRegistration(s), nil
	}
	if !onlyDigits(s) || len(s) < 2 || len(s) > 14 {
		return "", domain.ErrInvalidStateReg
	}
	return StateRegistration(s), nil
}

// Environment é o tpAmb: homologação (2) ou produção (1).
type Environment string

const (
	Homologation Environment = "homologation"
	Production   Environment = "production"
)

// ParseEnvironment confere o ambiente.
func ParseEnvironment(raw string) (Environment, error) {
	switch Environment(raw) {
	case Homologation, Production:
		return Environment(raw), nil
	}
	return "", domain.ErrInvalidEnvironment
}

// TpAmb é o valor do XML.
func (e Environment) TpAmb() int {
	if e == Production {
		return 1
	}
	return 2
}

func onlyDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

func itoa2(n int) string { return string([]byte{byte('0' + n/10), byte('0' + n%10)}) }
