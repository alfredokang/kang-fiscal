package accesskey_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/accesskey"
)

var issued = time.Date(2026, time.November, 8, 10, 0, 0, 0, time.UTC)

func TestBuildEParse(t *testing.T) {
	cases := []struct {
		name string
		p    accesskey.Parts
		want string
	}{
		{
			name: "NF-e de SP com CNPJ numérico",
			p: accesskey.Parts{UFCode: 35, Issued: issued, Document: "11222333000181",
				Model: accesskey.NFe, Series: 1, Number: 123, EmissionType: 1, Code: 12345678},
			want: "3526111122233300018155001000000123112345678",
		},
		{
			name: "NFC-e de MG com CNPJ alfanumérico",
			p: accesskey.Parts{UFCode: 31, Issued: issued, Document: "12ABC34501DE35",
				Model: accesskey.NFCe, Series: 2, Number: 999_999_999, EmissionType: 9, Code: 7},
			want: "312611" + "12ABC34501DE35" + "65" + "002" + "999999999" + "9" + "00000007",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			key, err := accesskey.Build(c.p)
			if err != nil {
				t.Fatal(err)
			}
			if len(key) != accesskey.Length || string(key)[:43] != c.want {
				t.Fatalf("chave %s, esperava o prefixo %s", key, c.want)
			}
			back, err := accesskey.Parse(key.String())
			if err != nil || back != key {
				t.Fatalf("Parse(%s) = %s, %v", key, back, err)
			}
			if key.Model() != c.p.Model || key.EmissionType() != c.p.EmissionType {
				t.Errorf("modelo=%d tpEmis=%d", key.Model(), key.EmissionType())
			}
		})
	}
}

// Dígito conferido com uma implementação independente: soma ponderada 2..9 da direita para
// a esquerda = 488; 488 mod 11 = 4; 11 − 4 = 7.
func TestDigitoConhecido(t *testing.T) {
	key, _ := accesskey.Build(accesskey.Parts{UFCode: 35, Issued: issued, Document: "11222333000181",
		Model: accesskey.NFe, Series: 1, Number: 123, EmissionType: 1, Code: 12345678})
	if got := string(key)[43]; got != '7' {
		t.Errorf("dígito: %c", got)
	}
}

func TestParseRecusa(t *testing.T) {
	key, _ := accesskey.Build(accesskey.Parts{UFCode: 35, Issued: issued, Document: "11222333000181",
		Model: accesskey.NFe, Series: 1, Number: 1, EmissionType: 1, Code: 1})
	s := key.String()

	wrongDigit := s[:43] + string("0123456789"[(int(s[43]-'0')+1)%10])
	letterOutsideCNPJ := "A" + s[1:]
	for _, in := range []string{wrongDigit, letterOutsideCNPJ, s[:43], ""} {
		if _, err := accesskey.Parse(in); !errors.Is(err, accesskey.ErrInvalid) {
			t.Errorf("Parse(%q) deveria recusar, veio %v", in, err)
		}
	}
	// Prefixo NFe e espaços, como vem copiado do DANFE, são aceitos.
	if _, err := accesskey.Parse("NFe" + s[:22] + " " + s[22:]); err != nil {
		t.Errorf("chave com prefixo e espaço: %v", err)
	}
}

func TestBuildRecusaCampoForaDaFaixa(t *testing.T) {
	ok := accesskey.Parts{UFCode: 35, Issued: issued, Document: "11222333000181",
		Model: accesskey.NFe, Series: 1, Number: 1, EmissionType: 1, Code: 1}
	mutations := map[string]func(*accesskey.Parts){
		"UF inexistente":   func(p *accesskey.Parts) { p.UFCode = 99 },
		"modelo 59":        func(p *accesskey.Parts) { p.Model = 59 },
		"número zero":      func(p *accesskey.Parts) { p.Number = 0 },
		"série 1000":       func(p *accesskey.Parts) { p.Series = 1000 },
		"cNF de 9 dígitos": func(p *accesskey.Parts) { p.Code = 100_000_000 },
		"documento curto":  func(p *accesskey.Parts) { p.Document = "123" },
	}
	for name, mutate := range mutations {
		p := ok
		mutate(&p)
		if _, err := accesskey.Build(p); !errors.Is(err, accesskey.ErrField) {
			t.Errorf("%s: esperava ErrField, veio %v", name, err)
		}
	}
}

func TestNewCodeNuncaIgualAoNumero(t *testing.T) {
	for range 1000 {
		code, err := accesskey.NewCode(42)
		if err != nil || code == 42 || code < 0 || code > 99_999_999 {
			t.Fatalf("cNF %d, %v", code, err)
		}
	}
}
