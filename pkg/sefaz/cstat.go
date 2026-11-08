package sefaz

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"sync"
)

//go:embed cstat.json
var cstatJSON []byte

// Code é a descrição de um código de retorno da SEFAZ.
type Code struct {
	Message string `json:"message"`
	// Hint é a dica de correção, quando há. É o que transforma "Rejeição 778" em algo que
	// o empresário consegue resolver sem ligar para o contador.
	Hint string `json:"hint"`
}

var (
	codesOnce sync.Once
	codes     map[string]Code
)

// Describe devolve a mensagem e a dica de um cStat. O xMotivo que a SEFAZ devolve é a
// fonte de verdade; isto complementa, não substitui.
func Describe(cStat int) (Code, bool) {
	codesOnce.Do(func() {
		var raw struct {
			Codes map[string]Code `json:"codes"`
		}
		_ = json.Unmarshal(cstatJSON, &raw)
		codes = raw.Codes
	})
	c, ok := codes[strconv.Itoa(cStat)]
	return c, ok
}
