package secretbox

import (
	"errors"
	"strings"
	"testing"
)

const chave = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // 32 bytes, só para o teste

func TestCifraEDecifra(t *testing.T) {
	b, err := New(chave)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := b.Seal("codigo-de-acesso")
	c, _ := b.Seal("codigo-de-acesso")
	if a == c || strings.Contains(a, "codigo") {
		t.Fatalf("cifra repetida ou legível: %s %s", a, c)
	}
	if got, err := b.Open(a); err != nil || got != "codigo-de-acesso" {
		t.Fatalf("abrir: %q %v", got, err)
	}
	mexido := a[:len(a)-4] + "AAAA"
	if _, err := b.Open(mexido); !errors.Is(err, ErrCorrupted) {
		t.Fatalf("valor alterado: %v", err)
	}
	outra, _ := New("ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=")
	if _, err := outra.Open(a); !errors.Is(err, ErrCorrupted) {
		t.Fatalf("outra chave: %v", err)
	}
}

func TestSemChaveRecusa(t *testing.T) {
	b, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Seal("x"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("sem chave: %v", err)
	}
	if _, err := New("curta"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("chave inválida: %v", err)
	}
}
