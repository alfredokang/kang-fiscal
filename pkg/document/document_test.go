package document_test

import (
	"errors"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/document"
)

func TestParseCNPJ(t *testing.T) {
	valid := map[string]string{
		"11.222.333/0001-81": "11222333000181",
		"11222333000181":     "11222333000181",
		// O exemplo da própria Receita para o CNPJ alfanumérico.
		"12.ABC.345/01DE-35": "12ABC34501DE35",
		"12abc34501de35":     "12ABC34501DE35", // minúscula é normalizada
	}
	for in, want := range valid {
		got, err := document.ParseCNPJ(in)
		if err != nil || string(got) != want {
			t.Errorf("ParseCNPJ(%q) = %q, %v; esperava %q", in, got, err, want)
		}
	}

	invalid := []string{
		"11.222.333/0001-82", // dígito errado
		"12.ABC.345/01DE-36",
		"00.000.000/0000-00", // dígito confere, documento não existe
		"1122233300018",      // curto
		"12.ABC.345/01DE-3A", // dígito verificador nunca é letra
		"12.AB#.345/01DE-35",
		"",
	}
	for _, in := range invalid {
		if _, err := document.ParseCNPJ(in); !errors.Is(err, document.ErrInvalidCNPJ) {
			t.Errorf("ParseCNPJ(%q) deveria ser inválido, veio %v", in, err)
		}
	}
}

func TestCNPJ(t *testing.T) {
	c, _ := document.ParseCNPJ("12ABC34501DE35")
	if !c.IsAlphanumeric() || c.Root() != "12ABC345" || c.Formatted() != "12.ABC.345/01DE-35" {
		t.Errorf("alfanumérico=%v raiz=%s formatado=%s", c.IsAlphanumeric(), c.Root(), c.Formatted())
	}
	n, _ := document.ParseCNPJ("11222333000181")
	if n.IsAlphanumeric() {
		t.Error("CNPJ numérico marcado como alfanumérico")
	}
}

func TestParseCPF(t *testing.T) {
	for _, in := range []string{"529.982.247-25", "52998224725"} {
		got, err := document.ParseCPF(in)
		if err != nil || got.Formatted() != "529.982.247-25" {
			t.Errorf("ParseCPF(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"529.982.247-26", "111.111.111-11", "5299822472", "5299822472A", ""} {
		if _, err := document.ParseCPF(in); !errors.Is(err, document.ErrInvalidCPF) {
			t.Errorf("ParseCPF(%q) deveria ser inválido, veio %v", in, err)
		}
	}
}
