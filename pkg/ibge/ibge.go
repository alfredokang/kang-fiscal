// Package ibge traz a tabela de municípios do IBGE: o código de 7 dígitos que vai no XML
// (cMun) e o nome e a UF que vão no papel (DANFE, DANFSe).
//
// Fonte: API de localidades do IBGE (servicodados.ibge.gov.br/api/v1/localidades/municipios),
// dado público. Para atualizar: scripts/ibge-municipios.sh.
package ibge

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed municipios.csv
var raw string

// Municipality é um município.
type Municipality struct {
	Code, Name, UF string
}

var (
	once  sync.Once
	table map[string]Municipality
)

func load() {
	table = make(map[string]Municipality, 5600)
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		p := strings.Split(line, ";")
		if len(p) == 3 {
			table[p[0]] = Municipality{Code: p[0], Name: p[1], UF: p[2]}
		}
	}
}

// Lookup devolve o município pelo código IBGE.
func Lookup(code string) (Municipality, bool) {
	once.Do(load)
	m, ok := table[strings.TrimSpace(code)]
	return m, ok
}

// Count é o número de municípios da tabela.
func Count() int {
	once.Do(load)
	return len(table)
}

var ufByCode = map[string]string{
	"11": "RO", "12": "AC", "13": "AM", "14": "RR", "15": "PA", "16": "AP", "17": "TO", "21": "MA", "22": "PI",
	"23": "CE", "24": "RN", "25": "PB", "26": "PE", "27": "AL", "28": "SE", "29": "BA", "31": "MG", "32": "ES",
	"33": "RJ", "35": "SP", "41": "PR", "42": "SC", "43": "RS", "50": "MS", "51": "MT", "52": "GO", "53": "DF",
}

// UF devolve a sigla da UF pelos dois primeiros dígitos do código IBGE (do município ou da UF).
func UF(code string) string {
	if len(code) < 2 {
		return ""
	}
	return ufByCode[code[:2]]
}
