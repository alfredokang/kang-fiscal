package pdf

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"testing"
)

// Lê de volta as barras desenhadas no fluxo do PDF (os retângulos cheios), converte as
// larguras em módulos e decodifica o Code 128 C: início, pares de dígitos, dígito de
// controle e parada. É o que um leitor faria — e não usa o caminho de codificação.
func TestCodigoDeBarrasDecodificaDeVolta(t *testing.T) {
	const key = "43180906929383000163550010000000261000010301"
	pg := New().AddPageA4()
	if err := pg.Barcode128C(10, 10, 100, 12, key); err != nil {
		t.Fatal(err)
	}

	re := regexp.MustCompile(`([0-9.]+) ([0-9.]+) ([0-9.]+) ([0-9.]+) re\nf`)
	type bar struct{ x, w float64 }
	var bars []bar
	for _, m := range re.FindAllStringSubmatch(pg.content.String(), -1) {
		x, _ := strconv.ParseFloat(m[1], 64)
		w, _ := strconv.ParseFloat(m[3], 64)
		bars = append(bars, bar{x, w})
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].x < bars[j].x })

	// Sequência barra/espaço em larguras absolutas, e o módulo como a menor largura.
	var widths []float64
	for i, b := range bars {
		if i > 0 {
			gap := b.x - (bars[i-1].x + bars[i-1].w)
			widths = append(widths, gap)
		}
		widths = append(widths, b.w)
	}
	module := math.MaxFloat64
	for _, w := range widths {
		if w > 0.01 && w < module {
			module = w
		}
	}
	var mods string
	for _, w := range widths {
		mods += strconv.Itoa(int(math.Round(w / module)))
	}

	lookup := map[string]int{}
	for v, p := range code128Patterns[:106] {
		lookup[p] = v
	}
	var values []int
	for len(mods) >= 6 {
		if len(mods) == 7 { // parada: 2331112
			if mods != code128Patterns[code128Stop] {
				t.Fatalf("parada inválida: %s", mods)
			}
			break
		}
		v, ok := lookup[mods[:6]]
		if !ok {
			t.Fatalf("padrão desconhecido %s", mods[:6])
		}
		values = append(values, v)
		mods = mods[6:]
	}
	if values[0] != code128StartC {
		t.Fatalf("não começa no modo C: %d", values[0])
	}
	sum := values[0]
	for i, v := range values[1 : len(values)-1] {
		sum += v * (i + 1)
	}
	if sum%103 != values[len(values)-1] {
		t.Fatalf("dígito de controle: %d, esperava %d", values[len(values)-1], sum%103)
	}
	var got string
	for _, v := range values[1 : len(values)-1] {
		got += fmt.Sprintf("%02d", v)
	}
	if got != key {
		t.Errorf("decodificado %s, esperava %s", got, key)
	}
}
