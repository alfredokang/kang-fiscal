// Package decimal é o número de ponto fixo de todo valor fiscal do Kang Fiscal.
//
// O leiaute da NF-e mistura escalas: vProd tem 2 casas, qCom até 4, vUnCom até 10 e as
// alíquotas 4. E a SEFAZ recalcula os totais e rejeita a nota por um centavo. Com float64, a
// divergência aparece só em alguns valores — e o bug que não se reproduz é o pior bug
// fiscal que existe. Aqui todo número é um inteiro e uma escala: 12,34 é {1234, 2}.
//
// O pacote não depende de nada além da biblioteca padrão.
package decimal

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// MaxScale é a maior escala aceita: a do vUnCom do leiaute 4.00.
const MaxScale = 10

var (
	// ErrInvalid: o texto não é um número decimal.
	ErrInvalid = errors.New("decimal: número inválido")
	// ErrTooPrecise: o texto tem mais casas do que a escala do campo. Não se arredonda em
	// silêncio na entrada — quem mandou 10,555 num campo de 2 casas precisa saber.
	ErrTooPrecise = errors.New("decimal: mais casas decimais do que o campo aceita")
	// ErrScale: escala fora de 0..MaxScale.
	ErrScale = errors.New("decimal: escala inválida")
	// ErrOverflow: o resultado não cabe em int64.
	ErrOverflow = errors.New("decimal: valor grande demais")
)

// Fixed é um número de ponto fixo: units × 10^-scale.
//
// O valor zero é 0 com escala 0 e é válido.
type Fixed struct {
	units int64
	scale uint8
}

// Rounding é a regra de arredondamento.
//
// Existem duas porque o manual não usa uma só: cada campo diz a sua, e é quem monta o
// campo que escolhe. Nenhuma função deste pacote arredonda sem receber a regra.
type Rounding int

const (
	// HalfUp: 0,5 vai para longe do zero. É a regra da maior parte dos campos de valor.
	HalfUp Rounding = iota
	// HalfEven: 0,5 vai para o par (ABNT NBR 5891). Evita o viés para cima em somas
	// longas de valores arredondados.
	HalfEven
)

var pow10 = [MaxScale + 1]int64{1, 10, 100, 1_000, 10_000, 100_000, 1_000_000,
	10_000_000, 100_000_000, 1_000_000_000, 10_000_000_000}

// New monta um número a partir das unidades e da escala: New(1234, 2) é 12,34.
func New(units int64, scale uint8) (Fixed, error) {
	if scale > MaxScale {
		return Fixed{}, ErrScale
	}
	return Fixed{units: units, scale: scale}, nil
}

// MustNew é New para constantes conhecidas em tempo de compilação. Entra em pânico com
// escala inválida — nunca use com valor que veio de fora.
func MustNew(units int64, scale uint8) Fixed {
	f, err := New(units, scale)
	if err != nil {
		panic(err)
	}
	return f
}

// Parse lê "12.34", "12,34", "-0.5" ou "7" na escala do campo.
//
// Aceita vírgula porque é como o contador escreve; aceita menos casas que a escala ("12.3"
// em escala 2 vira 12,30). Recusa mais casas com ErrTooPrecise, separador de milhar e
// notação científica — "1e3" num campo de valor é sempre engano de quem serializou com
// float.
func Parse(text string, scale uint8) (Fixed, error) {
	if scale > MaxScale {
		return Fixed{}, ErrScale
	}
	s := strings.TrimSpace(text)
	neg := false
	if strings.HasPrefix(s, "-") {
		neg, s = true, s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}

	intPart, fracPart, hasSep := strings.Cut(strings.Replace(s, ",", ".", 1), ".")
	if intPart == "" || (hasSep && fracPart == "") || !digits(intPart) || !digits(fracPart) {
		return Fixed{}, ErrInvalid
	}
	if len(fracPart) > int(scale) {
		// Zeros à direita além da escala não mudam o valor: "10.500" em escala 2 é 10,50.
		extra := fracPart[scale:]
		if strings.Trim(extra, "0") != "" {
			return Fixed{}, ErrTooPrecise
		}
		fracPart = fracPart[:scale]
	}
	fracPart += strings.Repeat("0", int(scale)-len(fracPart))

	n, ok := new(big.Int).SetString(intPart+fracPart, 10)
	if !ok {
		return Fixed{}, ErrInvalid
	}
	if neg {
		n.Neg(n)
	}
	if !n.IsInt64() {
		return Fixed{}, ErrOverflow
	}
	return Fixed{units: n.Int64(), scale: scale}, nil
}

// ParseAuto lê o número na escala que o próprio texto traz: "12.340" vira {12340, 3}.
// É o que o JSON de entrada usa antes de o domínio levar o valor à escala do campo.
func ParseAuto(text string) (Fixed, error) {
	s := strings.TrimSpace(text)
	_, frac, _ := strings.Cut(strings.Replace(s, ",", ".", 1), ".")
	if len(frac) > MaxScale {
		return Fixed{}, ErrTooPrecise
	}
	return Parse(s, uint8(len(frac)))
}

func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Units devolve o inteiro interno. Para centavos, use em escala 2.
func (f Fixed) Units() int64 { return f.units }

// Scale devolve a escala.
func (f Fixed) Scale() uint8 { return f.scale }

// IsZero diz se o valor é zero, em qualquer escala.
func (f Fixed) IsZero() bool { return f.units == 0 }

// IsNegative diz se o valor é menor que zero.
func (f Fixed) IsNegative() bool { return f.units < 0 }

// String formata com exatamente `scale` casas e ponto decimal — o formato do XML.
// Nunca usa notação científica nem separador de milhar.
func (f Fixed) String() string {
	n := new(big.Int).SetInt64(f.units)
	neg := n.Sign() < 0
	s := n.Abs(n).String()
	if f.scale > 0 {
		if len(s) <= int(f.scale) {
			s = strings.Repeat("0", int(f.scale)-len(s)+1) + s
		}
		cut := len(s) - int(f.scale)
		s = s[:cut] + "." + s[cut:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

// Rescale leva o número a outra escala. Aumentar a escala é exato; diminuir arredonda
// com a regra dada.
func (f Fixed) Rescale(scale uint8, mode Rounding) (Fixed, error) {
	if scale > MaxScale {
		return Fixed{}, ErrScale
	}
	if scale == f.scale {
		return f, nil
	}
	n := new(big.Int).SetInt64(f.units)
	if scale > f.scale {
		n.Mul(n, big.NewInt(pow10[scale-f.scale]))
	} else {
		n = divRound(n, big.NewInt(pow10[f.scale-scale]), mode)
	}
	if !n.IsInt64() {
		return Fixed{}, ErrOverflow
	}
	return Fixed{units: n.Int64(), scale: scale}, nil
}

// Add soma. O resultado fica na maior das duas escalas — somar é sempre exato.
func (f Fixed) Add(o Fixed) (Fixed, error) {
	return f.combine(o, func(a, b *big.Int) *big.Int { return a.Add(a, b) })
}

// Sub subtrai, na maior das duas escalas.
func (f Fixed) Sub(o Fixed) (Fixed, error) {
	return f.combine(o, func(a, b *big.Int) *big.Int { return a.Sub(a, b) })
}

func (f Fixed) combine(o Fixed, op func(a, b *big.Int) *big.Int) (Fixed, error) {
	scale := max(f.scale, o.scale)
	a, _ := f.Rescale(scale, HalfUp) // aumentar a escala nunca arredonda
	b, _ := o.Rescale(scale, HalfUp)
	n := op(big.NewInt(a.units), big.NewInt(b.units))
	if !n.IsInt64() {
		return Fixed{}, ErrOverflow
	}
	return Fixed{units: n.Int64(), scale: scale}, nil
}

// Mul multiplica e arredonda **uma vez**, direto na escala do resultado.
//
// Quantidade × valor unitário (4 casas × 10 casas) tem 14 casas antes de chegar às 2 do
// vProd; arredondar no meio do caminho é o centavo que a SEFAZ acusa (Constituição, Art.
// 5.2). O cálculo intermediário é em big.Int, então não há estouro escondido.
func (f Fixed) Mul(o Fixed, scale uint8, mode Rounding) (Fixed, error) {
	if scale > MaxScale {
		return Fixed{}, ErrScale
	}
	n := new(big.Int).Mul(big.NewInt(f.units), big.NewInt(o.units))
	from := int(f.scale) + int(o.scale)
	n, err := toScale(n, from, int(scale), mode)
	if err != nil {
		return Fixed{}, err
	}
	return Fixed{units: n.Int64(), scale: scale}, nil
}

// Percent aplica uma alíquota em percentual: base × (alíquota / 100), na escala dada.
// É o cálculo de todo tributo do leiaute (vICMS = vBC × pICMS / 100, vCBS = vBC × pCBS / 100).
func (f Fixed) Percent(rate Fixed, scale uint8, mode Rounding) (Fixed, error) {
	if scale > MaxScale {
		return Fixed{}, ErrScale
	}
	n := new(big.Int).Mul(big.NewInt(f.units), big.NewInt(rate.units))
	// Dividir por 100 é somar 2 à escala de origem.
	from := int(f.scale) + int(rate.scale) + 2
	n, err := toScale(n, from, int(scale), mode)
	if err != nil {
		return Fixed{}, err
	}
	return Fixed{units: n.Int64(), scale: scale}, nil
}

// toScale leva n (na escala `from`, que pode passar de MaxScale) para `to`.
func toScale(n *big.Int, from, to int, mode Rounding) (*big.Int, error) {
	switch {
	case from > to:
		div := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(from-to)), nil)
		n = divRound(n, div, mode)
	case from < to:
		mul := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(to-from)), nil)
		n = n.Mul(n, mul)
	}
	if !n.IsInt64() {
		return nil, ErrOverflow
	}
	return n, nil
}

// divRound divide n por d (d > 0) arredondando com a regra dada.
func divRound(n, d *big.Int, mode Rounding) *big.Int {
	q, r := new(big.Int).QuoRem(n, d, new(big.Int))
	if r.Sign() == 0 {
		return q
	}
	// Compara 2·|resto| com o divisor: menor arredonda para dentro, maior para fora, igual
	// é o empate que a regra decide.
	twice := new(big.Int).Abs(r)
	twice.Lsh(twice, 1)
	away := false
	switch twice.Cmp(d) {
	case 1:
		away = true
	case 0:
		away = mode == HalfUp || q.Bit(0) == 1
	}
	if away {
		if n.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q
}

// Cmp compara: -1, 0 ou 1. Escalas diferentes são comparadas pelo valor, não pelo inteiro.
func (f Fixed) Cmp(o Fixed) int {
	scale := max(f.scale, o.scale)
	a := new(big.Int).Mul(big.NewInt(f.units), big.NewInt(pow10[scale-f.scale]))
	b := new(big.Int).Mul(big.NewInt(o.units), big.NewInt(pow10[scale-o.scale]))
	return a.Cmp(b)
}

// MarshalJSON escreve o número como string ("12.34").
//
// String, e não número JSON, porque o JavaScript de quem consome a API lê número como
// float64 — e o valor que voltasse num PUT já não seria o mesmo.
func (f Fixed) MarshalJSON() ([]byte, error) {
	return []byte(`"` + f.String() + `"`), nil
}

// UnmarshalJSON aceita "12.34" e 12.34. O número JSON é lido pelo texto, nunca convertido
// para float no caminho.
func (f *Fixed) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "null" || s == "" {
		*f = Fixed{}
		return nil
	}
	v, err := ParseAuto(s)
	if err != nil {
		return fmt.Errorf("%w: %q", err, s)
	}
	*f = v
	return nil
}
