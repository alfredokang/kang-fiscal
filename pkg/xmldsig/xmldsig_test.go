package xmldsig_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

// Documentos assinados por terceiros e aceitos pela SEFAZ (testdata/README.md). Conferir a
// assinatura deles prova que a canonicalização daqui produz os mesmos bytes que a de quem
// emitiu — o teste que nenhum documento gerado por nós mesmos pode substituir.
func TestConfereDocumentosReais(t *testing.T) {
	cases := map[string]string{
		"nfe_310_sp_assinada.xml":   "",
		"nfe_400_rs_autorizada.xml": "",
		"nfce_400_ba_assinada.xml":  "",
		// O evento vem dentro do nfeDadosMsg, com outro namespace default acima: prova que
		// o ápice herda o namespace certo.
		"evento_cce_sp.xml": "",
	}
	for name := range cases {
		t.Run(name, func(t *testing.T) {
			doc, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			cert, err := xmldsig.Verify(doc)
			if err != nil {
				t.Fatalf("assinatura real recusada: %v", err)
			}
			if cert.Subject.CommonName == "" {
				t.Error("certificado sem titular")
			}

			// Um caractere a mais dentro do bloco assinado tem de derrubar a assinatura.
			tampered := bytes.Replace(doc, []byte("<cUF>"), []byte("<cUF>9"), 1)
			if bytes.Equal(tampered, doc) {
				tampered = bytes.Replace(doc, []byte("<tpAmb>2"), []byte("<tpAmb>1"), 1)
			}
			if _, err := xmldsig.Verify(tampered); !errors.Is(err, xmldsig.ErrInvalid) {
				t.Errorf("documento alterado passou: %v", err)
			}
		})
	}
}

const unsigned = `<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe35261111222333000181550010000001231123456787" versao="4.00"><ide><cUF>35</cUF><natOp>VENDA &amp; CIA &lt;teste&gt;</natOp></ide><emit><xNome>Café Exemplo</xNome></emit></infNFe></NFe>`

func testSigner(t *testing.T) *certificate.Certificate {
	t.Helper()
	pfx, err := certificate.TestPFX("CAFE EXEMPLO LTDA", "11222333000181", "s", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	c, err := certificate.Load(pfx, "s")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAssinaEConfere(t *testing.T) {
	signer := testSigner(t)
	signed, err := xmldsig.Sign([]byte(unsigned), "infNFe", signer)
	if err != nil {
		t.Fatal(err)
	}
	s := string(signed)

	// O Signature é o último filho do NFe, e o resto do documento continua byte a byte.
	if !strings.HasSuffix(s, "</Signature></NFe>") {
		t.Errorf("Signature fora do lugar: ...%s", s[len(s)-60:])
	}
	if !strings.HasPrefix(s, unsigned[:len(unsigned)-len("</NFe>")]) {
		t.Error("a assinatura reserializou o documento")
	}
	if !strings.Contains(s, `<Reference URI="#NFe35261111222333000181550010000001231123456787">`) {
		t.Error("Reference sem o Id")
	}

	cert, err := xmldsig.Verify(signed)
	if err != nil {
		t.Fatalf("a própria assinatura não confere: %v", err)
	}
	if cert.Subject.CommonName != "CAFE EXEMPLO LTDA:11222333000181" {
		t.Errorf("certificado: %s", cert.Subject.CommonName)
	}

	if _, err := xmldsig.Sign(signed, "infNFe", signer); !errors.Is(err, xmldsig.ErrAlreadySigned) {
		t.Errorf("assinar duas vezes: %v", err)
	}
}

func TestRecusas(t *testing.T) {
	signer := testSigner(t)
	cases := map[string]struct {
		doc  string
		want error
	}{
		"sem o elemento":  {`<NFe><outro Id="x"/></NFe>`, xmldsig.ErrElementNotFound},
		"sem Id":          {`<NFe><infNFe versao="4.00"></infNFe></NFe>`, xmldsig.ErrNoID},
		"DOCTYPE (XXE)":   {`<!DOCTYPE x [<!ENTITY e SYSTEM "file:///etc/passwd">]><NFe><infNFe Id="a">&e;</infNFe></NFe>`, xmldsig.ErrForbidden},
		"malformado":      {`<NFe><infNFe Id="a"></NFe>`, xmldsig.ErrMalformed},
		"duas raízes":     {`<a></a><b></b>`, xmldsig.ErrMalformed},
		"entidade avulsa": {`<NFe><infNFe Id="a">&e;</infNFe></NFe>`, xmldsig.ErrMalformed},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := xmldsig.Sign([]byte(c.doc), "infNFe", signer); !errors.Is(err, c.want) {
				t.Errorf("esperava %v, veio %v", c.want, err)
			}
		})
	}
	if _, err := xmldsig.Verify([]byte(unsigned)); !errors.Is(err, xmldsig.ErrNotSigned) {
		t.Errorf("documento sem assinatura: %v", err)
	}
}

// Um algoritmo fora do perfil (SHA-256 no lugar de SHA-1, por exemplo) não pode passar
// como válido só porque a matemática fecha: a SEFAZ recusaria.
func TestAlgoritmoForaDoPerfil(t *testing.T) {
	signed, _ := xmldsig.Sign([]byte(unsigned), "infNFe", testSigner(t))
	swapped := bytes.Replace(signed, []byte(xmldsig.AlgDigest), []byte("http://www.w3.org/2001/04/xmlenc#sha256"), 1)
	if _, err := xmldsig.Verify(swapped); !errors.Is(err, xmldsig.ErrAlgorithm) {
		t.Errorf("algoritmo trocado: %v", err)
	}
}
