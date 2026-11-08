package danfe_test

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/danfe"
)

func TestDANFSe(t *testing.T) {
	doc, err := os.ReadFile("testdata/nfse.xml")
	if err != nil {
		t.Fatal(err)
	}
	out, err := danfe.GenerateNFSe(doc, danfe.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-1.4")) {
		t.Fatalf("não é PDF")
	}
	// O texto vai no PDF em WinAnsi, sem compressão: dá para procurar os campos.
	for _, want := range []string{"DANFSe v2.0", "35503082211222333000181000000000001726110000000017", "CLIENTE LTDA",
		"CAFE EXEMPLO LTDA", "R$ 1.500,00", "R$ 13,50", "000 / 000001", "NFS-e SEM VALIDADE JUR", "01.07.01"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("falta %q no DANFSe", want)
		}
	}
	if os.Getenv("DANFSE_OUT") != "" {
		_ = os.WriteFile(os.Getenv("DANFSE_OUT"), out, 0o600)
	}
	canceled, _ := danfe.GenerateNFSe(doc, danfe.Options{Canceled: true})
	if !bytes.Contains(canceled, []byte("CANCELADA")) {
		t.Error("carimbo de cancelada")
	}
}

func TestDANFSeRecusaOutroXML(t *testing.T) {
	if _, err := danfe.GenerateNFSe([]byte(`<nfeProc/>`), danfe.Options{}); !errors.Is(err, danfe.ErrNotNFSe) {
		t.Errorf("NF-e no lugar da NFS-e: %v", err)
	}
}
