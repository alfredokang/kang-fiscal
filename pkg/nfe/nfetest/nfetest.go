// Package nfetest ajuda a testar quem gera NF-e: monta a identificação, assina com um
// certificado de teste e valida contra o XSD oficial.
package nfetest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/accesskey"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

// CNPJ é o emitente dos testes.
const CNPJ = "11222333000181"

// Emit é um emitente de SP no regime dado (CRT).
func Emit(crt int) nfe.Emit {
	return nfe.Emit{
		CNPJ: CNPJ, XNome: "CAFE EXEMPLO LTDA",
		EnderEmit: nfe.Endereco{XLgr: "RUA DAS FLORES", Nro: "100", XBairro: "CENTRO", CMun: "3550308",
			XMun: "SAO PAULO", UF: "SP", CEP: "01001000", CPais: "1058", XPais: "BRASIL"},
		IE: "123456789110", CRT: crt,
	}
}

// Ide monta a identificação e devolve também o Id do infNFe.
func Ide(t testing.TB, model, number int) (nfe.Ide, string) {
	t.Helper()
	code, _ := accesskey.NewCode(number)
	issued := time.Date(2026, 11, 8, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	key, err := accesskey.Build(accesskey.Parts{UFCode: 35, Issued: issued, Document: CNPJ,
		Model: accesskey.Model(model), Series: 1, Number: number, EmissionType: 1, Code: code})
	if err != nil {
		t.Fatal(err)
	}
	tpImp, indPres := 1, 2
	if model == 65 {
		tpImp, indPres = 4, 1
	}
	k := key.String()
	return nfe.Ide{CUF: 35, CNF: k[35:43], NatOp: "VENDA DE MERCADORIA", Mod: model, Serie: 1, NNF: number,
		DhEmi: issued.Format("2006-01-02T15:04:05-07:00"), TpNF: 1, IdDest: 1, CMunFG: "3550308", TpImp: tpImp,
		TpEmis: 1, CDV: int(k[43] - '0'), TpAmb: 2, FinNFe: 1, IndFinal: 1, IndPres: indPres, ProcEmi: 0,
		VerProc: "kang-fiscal-test"}, "NFe" + k
}

// Sign serializa e assina com um certificado de teste, conferindo a própria assinatura.
func Sign(t testing.TB, n *nfe.NFe) []byte {
	t.Helper()
	raw, err := nfe.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	pfx, _ := certificate.TestPFX("CAFE EXEMPLO LTDA", CNPJ, "s", time.Now().Add(time.Hour))
	cert, _ := certificate.Load(pfx, "s")
	out, err := xmldsig.Sign(raw, "infNFe", cert)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xmldsig.Verify(out); err != nil {
		t.Fatal(err)
	}
	return out
}

// ValidateXSD valida contra o XSD oficial com o xmllint. Sem xmllint, pula o teste — o CI o
// instala (libxml2-utils).
func ValidateXSD(t testing.TB, doc []byte) ([]byte, error) {
	t.Helper()
	bin, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("xmllint não instalado: a validação contra o XSD oficial ficou de fora")
	}
	f := filepath.Join(t.TempDir(), "nfe.xml")
	if err := os.WriteFile(f, doc, 0o600); err != nil {
		t.Fatal(err)
	}
	return exec.Command(bin, "--noout", "--schema", SchemaPath(), f).CombinedOutput()
}

// MustValidateSchema valida contra outro schema do pacote (eventos: envEventoCancNFe_v1.00.xsd,
// envCCe_v1.00.xsd).
func MustValidateSchema(t testing.TB, doc []byte, schema string) {
	t.Helper()
	bin, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("xmllint não instalado: a validação contra o XSD oficial ficou de fora")
	}
	f := filepath.Join(t.TempDir(), "doc.xml")
	if err := os.WriteFile(f, doc, 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(SchemaPath()), schema)
	if out, err := exec.Command(bin, "--noout", "--schema", path, f).CombinedOutput(); err != nil {
		t.Fatalf("o XML não passa em %s:\n%s\n%s", schema, out, doc)
	}
}

// SignEvent assina o infEvento com o certificado de teste.
func SignEvent(t testing.TB, doc []byte) []byte {
	t.Helper()
	pfx, _ := certificate.TestPFX("CAFE EXEMPLO LTDA", CNPJ, "s", time.Now().Add(time.Hour))
	cert, _ := certificate.Load(pfx, "s")
	out, err := xmldsig.Sign(doc, "infEvento", cert)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// MustValidateXSD falha o teste se o XML não passa no schema.
func MustValidateXSD(t testing.TB, doc []byte) {
	t.Helper()
	if out, err := ValidateXSD(t, doc); err != nil {
		t.Fatalf("o XML não passa no XSD oficial:\n%s\n%s", out, doc)
	}
}

// SchemaPath é o nfe_v4.00.xsd do pacote vigente, relativo a este arquivo.
func SchemaPath() string {
	_, here, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(here), "..", "testdata", "schemas", "PL_010_V1.30", "nfe_v4.00.xsd")
}
