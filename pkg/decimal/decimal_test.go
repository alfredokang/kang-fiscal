package decimal_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/decimal"
)

func mustParse(t *testing.T, s string, scale uint8) decimal.Fixed {
	t.Helper()
	f, err := decimal.Parse(s, scale)
	if err != nil {
		t.Fatalf("Parse(%q, %d): %v", s, scale, err)
	}
	return f
}

func TestParse(t *testing.T) {
	cases := []struct {
		in    string
		scale uint8
		want  string
		err   error
	}{
		{"12.34", 2, "12.34", nil},
		{"12,34", 2, "12.34", nil}, // como o contador escreve
		{"12.3", 2, "12.30", nil},
		{"7", 4, "7.0000", nil},
		{"-0.5", 2, "-0.50", nil},
		{"10.500", 2, "10.50", nil}, // zero à direita não é precisão a mais
		{"0.0000000001", 10, "0.0000000001", nil},
		{"10.555", 2, "", decimal.ErrTooPrecise}, // nunca arredonda em silêncio
		{"1e3", 2, "", decimal.ErrInvalid},       // float serializado errado
		{"1.000,00", 2, "", decimal.ErrInvalid},  // separador de milhar
		{"", 2, "", decimal.ErrInvalid},
		{"12.", 2, "", decimal.ErrInvalid},
		{"abc", 2, "", decimal.ErrInvalid},
		{"1", 11, "", decimal.ErrScale},
		{"99999999999999999999", 0, "", decimal.ErrOverflow},
	}
	for _, c := range cases {
		got, err := decimal.Parse(c.in, c.scale)
		if !errors.Is(err, c.err) {
			t.Errorf("Parse(%q, %d): erro %v, esperava %v", c.in, c.scale, err, c.err)
			continue
		}
		if err == nil && got.String() != c.want {
			t.Errorf("Parse(%q, %d) = %s, esperava %s", c.in, c.scale, got, c.want)
		}
	}
}

// O caso clássico: em float64, 0.1 + 0.2 = 0.30000000000000004.
func TestSomaExata(t *testing.T) {
	a := mustParse(t, "0.1", 2)
	b := mustParse(t, "0.2", 2)
	got, err := a.Add(b)
	if err != nil || got.String() != "0.30" {
		t.Fatalf("0.1 + 0.2 = %s (%v)", got, err)
	}
	if got.Cmp(mustParse(t, "0.3", 1)) != 0 {
		t.Error("Cmp deveria comparar valor, não escala")
	}
}

// vProd = qCom × vUnCom, arredondado UMA vez na escala 2.
func TestMulArredondaUmaVez(t *testing.T) {
	q := mustParse(t, "3.0000", 4)
	vUn := mustParse(t, "0.3333333333", 10)
	got, err := q.Mul(vUn, 2, decimal.HalfUp)
	if err != nil || got.String() != "1.00" {
		t.Fatalf("3 × 0,3333333333 = %s (%v)", got, err)
	}

	// 2,5 kg × R$ 10,01 = 25,025 → 25,03 (meio para cima)
	got, _ = mustParse(t, "2.5", 4).Mul(mustParse(t, "10.01", 2), 2, decimal.HalfUp)
	if got.String() != "25.03" {
		t.Errorf("HalfUp: %s", got)
	}
	// ... e 25,02 no meio para o par.
	got, _ = mustParse(t, "2.5", 4).Mul(mustParse(t, "10.01", 2), 2, decimal.HalfEven)
	if got.String() != "25.02" {
		t.Errorf("HalfEven: %s", got)
	}
}

func TestPercent(t *testing.T) {
	cases := []struct {
		base, rate string
		mode       decimal.Rounding
		want       string
	}{
		{"100.00", "18.0000", decimal.HalfUp, "18.00"},
		// CBS de teste de 2026: 0,9% sobre R$ 33,33 = 0,29997 → 0,30
		{"33.33", "0.9000", decimal.HalfUp, "0.30"},
		// IBS de teste de 2026: 0,1% sobre R$ 5,00 = 0,005 → empate
		{"5.00", "0.1000", decimal.HalfUp, "0.01"},
		{"5.00", "0.1000", decimal.HalfEven, "0.00"},
		{"-10.00", "12.5000", decimal.HalfUp, "-1.25"},
		{"0.01", "7.5000", decimal.HalfUp, "0.00"},
	}
	for _, c := range cases {
		base := mustParse(t, c.base, 2)
		rate := mustParse(t, c.rate, 4)
		got, err := base.Percent(rate, 2, c.mode)
		if err != nil || got.String() != c.want {
			t.Errorf("%s × %s%% = %s (%v), esperava %s", c.base, c.rate, got, err, c.want)
		}
	}
}

func TestRescale(t *testing.T) {
	got, _ := mustParse(t, "1.005", 3).Rescale(2, decimal.HalfUp)
	if got.String() != "1.01" {
		t.Errorf("1,005 → %s", got)
	}
	got, _ = mustParse(t, "-1.005", 3).Rescale(2, decimal.HalfUp)
	if got.String() != "-1.01" {
		t.Errorf("-1,005 → %s (o meio vai para longe do zero)", got)
	}
	got, _ = mustParse(t, "1.5", 1).Rescale(4, decimal.HalfUp)
	if got.String() != "1.5000" {
		t.Errorf("1,5 → %s", got)
	}
}

func TestString(t *testing.T) {
	cases := map[string]decimal.Fixed{
		"0.05":   decimal.MustNew(5, 2),
		"-0.05":  decimal.MustNew(-5, 2),
		"0":      decimal.Fixed{},
		"123.40": decimal.MustNew(12340, 2),
		"0.0001": decimal.MustNew(1, 4),
	}
	for want, f := range cases {
		if f.String() != want {
			t.Errorf("String() = %s, esperava %s", f, want)
		}
	}
}

func TestJSONNuncaPassaPorFloat(t *testing.T) {
	var v struct {
		A decimal.Fixed `json:"a"`
		B decimal.Fixed `json:"b"`
	}
	// 0.1234567891 tem 10 casas: como float64 viraria 0.12345678910000000.
	if err := json.Unmarshal([]byte(`{"a": 0.1234567891, "b": "12,50"}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A.String() != "0.1234567891" || v.B.String() != "12.50" {
		t.Fatalf("a=%s b=%s", v.A, v.B)
	}
	out, _ := json.Marshal(v)
	if string(out) != `{"a":"0.1234567891","b":"12.50"}` {
		t.Errorf("saída: %s", out)
	}
	if err := json.Unmarshal([]byte(`{"a": 1e3}`), &v); err == nil {
		t.Error("notação científica deveria ser recusada")
	}
}
