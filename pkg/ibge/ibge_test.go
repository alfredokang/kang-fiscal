package ibge_test

import (
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/ibge"
)

func TestMunicipios(t *testing.T) {
	if n := ibge.Count(); n < 5570 {
		t.Errorf("tabela incompleta: %d municípios", n)
	}
	for code, want := range map[string]ibge.Municipality{
		"3550308": {Code: "3550308", Name: "São Paulo", UF: "SP"},
		"5300108": {Code: "5300108", Name: "Brasília", UF: "DF"},
		"4314902": {Code: "4314902", Name: "Porto Alegre", UF: "RS"},
	} {
		if got, ok := ibge.Lookup(code); !ok || got != want {
			t.Errorf("%s: %+v", code, got)
		}
	}
	if _, ok := ibge.Lookup("9999999"); ok {
		t.Error("código inexistente")
	}
	if ibge.UF("3106200") != "MG" {
		t.Error("UF pelo código")
	}
}
