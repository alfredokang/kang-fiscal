// Package document valida e normaliza CNPJ e CPF, incluindo o CNPJ alfanumérico.
//
// Formato certo com dígito errado é recusado pela SEFAZ na emissão, depois da venda — e
// é o erro mais comum de cadastro. Conferir o dígito aqui custa microssegundos e evita a
// nota rejeitada.
//
// O CNPJ alfanumérico (IN RFB nº 2.229/2024) passa a ser atribuído a partir de julho de
// 2026: as 12 primeiras posições aceitam letras de A a Z, os dois dígitos verificadores
// continuam numéricos, e o cálculo usa o código ASCII do caractere menos 48 — o que faz o
// CNPJ só com números dar exatamente o mesmo dígito de antes.
package document

import (
	"errors"
	"strings"
)

var (
	// ErrInvalidCNPJ: tamanho, caractere ou dígito verificador errado.
	ErrInvalidCNPJ = errors.New("CNPJ inválido")
	// ErrInvalidCPF: tamanho, caractere ou dígito verificador errado.
	ErrInvalidCPF = errors.New("CPF inválido")
)

// CNPJ é um CNPJ válido, só com os 14 caracteres, em maiúsculas.
type CNPJ string

// CPF é um CPF válido, só com os 11 dígitos.
type CPF string

// ParseCNPJ aceita o CNPJ com ou sem pontuação, numérico ou alfanumérico.
func ParseCNPJ(raw string) (CNPJ, error) {
	s := strings.ToUpper(strip(raw))
	if len(s) != 14 {
		return "", ErrInvalidCNPJ
	}
	for i := 0; i < 12; i++ {
		if !isDigit(s[i]) && (s[i] < 'A' || s[i] > 'Z') {
			return "", ErrInvalidCNPJ
		}
	}
	if !isDigit(s[12]) || !isDigit(s[13]) || allSame(s) {
		return "", ErrInvalidCNPJ
	}
	if CNPJCheckDigits(s[:12]) != s[12:] {
		return "", ErrInvalidCNPJ
	}
	return CNPJ(s), nil
}

// CNPJCheckDigits calcula os dois dígitos verificadores das 12 primeiras posições.
func CNPJCheckDigits(base string) string {
	d1 := mod11(base, []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2})
	d2 := mod11(base+string(d1), []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2})
	return string([]byte{d1, d2})
}

// IsAlphanumeric diz se o CNPJ tem letra. Alguns serviços antigos de terceiros só
// aceitam o numérico — quem integra precisa saber antes de mandar.
func (c CNPJ) IsAlphanumeric() bool {
	for i := 0; i < len(c); i++ {
		if !isDigit(c[i]) {
			return true
		}
	}
	return false
}

// Root é a raiz (8 primeiras posições): identifica a empresa, sem a filial.
func (c CNPJ) Root() string { return string(c)[:8] }

// Formatted devolve 12.ABC.345/01DE-35.
func (c CNPJ) Formatted() string {
	s := string(c)
	return s[0:2] + "." + s[2:5] + "." + s[5:8] + "/" + s[8:12] + "-" + s[12:14]
}

// ParseCPF aceita o CPF com ou sem pontuação.
func ParseCPF(raw string) (CPF, error) {
	s := strip(raw)
	if len(s) != 11 || allSame(s) {
		return "", ErrInvalidCPF
	}
	for i := 0; i < 11; i++ {
		if !isDigit(s[i]) {
			return "", ErrInvalidCPF
		}
	}
	d1 := mod11(s[:9], []int{10, 9, 8, 7, 6, 5, 4, 3, 2})
	d2 := mod11(s[:9]+string(d1), []int{11, 10, 9, 8, 7, 6, 5, 4, 3, 2})
	if s[9] != d1 || s[10] != d2 {
		return "", ErrInvalidCPF
	}
	return CPF(s), nil
}

// Formatted devolve 123.456.789-09.
func (c CPF) Formatted() string {
	s := string(c)
	return s[0:3] + "." + s[3:6] + "." + s[6:9] + "-" + s[9:11]
}

// mod11 é o módulo 11 da Receita: resto menor que 2 dá 0, senão 11 − resto. O valor de
// cada caractere é o código ASCII menos 48 ('0' = 0, 'A' = 17).
func mod11(s string, weights []int) byte {
	sum := 0
	for i, w := range weights {
		sum += int(s[i]-'0') * w
	}
	r := sum % 11
	if r < 2 {
		return '0'
	}
	return byte('0' + 11 - r)
}

// strip remove pontuação e espaço. Não remove letra: no CNPJ alfanumérico ela é dado.
func strip(raw string) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '.', '-', '/', ' ':
			continue
		}
		b.WriteByte(raw[i])
	}
	return b.String()
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// allSame recusa 000.000.000-00 e parecidos: o dígito confere, e o documento não existe.
func allSame(s string) bool {
	return strings.Count(s, s[:1]) == len(s)
}
