package certificate_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
)

func TestCarregaECnpjAlfanumerico(t *testing.T) {
	notAfter := time.Now().Add(365 * 24 * time.Hour).Truncate(time.Second)
	pfx, err := certificate.TestPFX("CAFE EXEMPLO LTDA", "12ABC34501DE35", "segredo", notAfter)
	if err != nil {
		t.Fatal(err)
	}
	c, err := certificate.Load(pfx, "segredo")
	if err != nil {
		t.Fatal(err)
	}
	// As letras do CNPJ alfanumérico não podem sumir na leitura do otherName.
	if c.CNPJ() != "12ABC34501DE35" {
		t.Errorf("CNPJ: %q", c.CNPJ())
	}
	if c.HolderName() != "CAFE EXEMPLO LTDA" || c.CPF() != "" {
		t.Errorf("titular=%q cpf=%q", c.HolderName(), c.CPF())
	}
	if err := c.ValidAt(time.Now()); err != nil {
		t.Error(err)
	}
	if err := c.ValidAt(notAfter.Add(time.Hour)); !errors.Is(err, certificate.ErrExpired) {
		t.Errorf("depois do vencimento: %v", err)
	}
	if len(c.Fingerprint()) != 64 || len(c.TLS().Certificate) != 1 {
		t.Errorf("fingerprint=%s", c.Fingerprint())
	}
}

func TestSenhaErrada(t *testing.T) {
	pfx, _ := certificate.TestPFX("X", "11222333000181", "certa", time.Now().Add(time.Hour))
	if _, err := certificate.Load(pfx, "errada"); !errors.Is(err, certificate.ErrWrongPassword) {
		t.Errorf("senha errada: %v", err)
	}
	if _, err := certificate.Load([]byte("não é pfx"), ""); !errors.Is(err, certificate.ErrWrongPassword) {
		t.Errorf("lixo: %v", err)
	}
}

// Constituição, Art. 9.1: o certificado nunca aparece num log, nem por engano.
func TestNuncaImprimeAChave(t *testing.T) {
	pfx, _ := certificate.TestPFX("X", "11222333000181", "s", time.Now().Add(time.Hour))
	c, _ := certificate.Load(pfx, "s")
	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		out := fmt.Sprintf(format, c)
		if out != "[certificado omitido]" || strings.Contains(out, "PRIVATE") {
			t.Errorf("%s imprimiu %q", format, out)
		}
	}
}
