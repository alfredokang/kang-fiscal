package danfe_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/danfe"
)

// pdfToText usa o pdftotext (poppler), quando instalado, para conferir que o PDF abre num
// leitor de verdade e que o texto está lá. Sem ele, só a estrutura é conferida.
func pdfToText(t *testing.T, doc []byte) string {
	t.Helper()
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		return ""
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "danfe.pdf")
	_ = os.WriteFile(in, doc, 0o600)
	out, err := exec.Command(bin, "-layout", in, "-").CombinedOutput()
	if err != nil {
		t.Fatalf("o PDF não abre: %v\n%s", err, out)
	}
	return string(out)
}

// Uma NF-e real autorizada (testdata do xmldsig, vinda do sped-nfe).
func TestDANFEDeNotaReal(t *testing.T) {
	doc, err := os.ReadFile("../xmldsig/testdata/nfe_400_rs_autorizada.xml")
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := danfe.Generate(doc, danfe.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(bytes.TrimSpace(pdf), []byte("%%EOF")) {
		t.Fatalf("não é um PDF: %.40q", pdf)
	}
	if err := os.WriteFile(filepath.Join(os.TempDir(), "kang-fiscal-danfe-teste.pdf"), pdf, 0o600); err == nil {
		t.Logf("DANFE gerado em %s", filepath.Join(os.TempDir(), "kang-fiscal-danfe-teste.pdf"))
	}
	if text := pdfToText(t, pdf); text != "" {
		for _, want := range []string{"DANFE", "4318 0906 9293 8300 0163 5500 1000 0000 2610 0001 0301", "CHAVE DE ACESSO", "PROTOCOLO DE AUTORIZAÇÃO"} {
			if !strings.Contains(text, want) {
				t.Errorf("o DANFE não mostra %q", want)
			}
		}
	}
}

func TestDANFERecusaQueNaoENFe(t *testing.T) {
	if _, err := danfe.Generate([]byte(`<outro/>`), danfe.Options{}); !errors.Is(err, danfe.ErrNotNFe) {
		t.Errorf("XML que não é NF-e: %v", err)
	}
}
