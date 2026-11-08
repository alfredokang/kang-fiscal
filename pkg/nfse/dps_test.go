package nfse_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfse"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

func d(s string) decimal.Fixed { v, _ := decimal.ParseAuto(s); return v }

func signed(t *testing.T, dps *nfse.DPS) []byte {
	t.Helper()
	raw, err := nfse.Marshal(dps)
	if err != nil {
		t.Fatal(err)
	}
	pfx, _ := certificate.TestPFX("CAFE EXEMPLO LTDA", "11222333000181", "s", time.Now().Add(time.Hour))
	cert, _ := certificate.Load(pfx, "s")
	out, err := xmldsig.Sign(raw, "infDPS", cert)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xmldsig.Verify(out); err != nil {
		t.Fatal(err)
	}
	return out
}

func validate(t *testing.T, doc []byte) {
	t.Helper()
	bin, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("xmllint não instalado")
	}
	f := filepath.Join(t.TempDir(), "dps.xml")
	_ = os.WriteFile(f, doc, 0o600)
	if out, err := exec.Command(bin, "--noout", "--schema", "testdata/schemas/20261020/DPS_v1.01.xsd", f).CombinedOutput(); err != nil {
		t.Fatalf("a DPS não passa no XSD:\n%s\n%s", out, doc)
	}
}

func base(t *testing.T) *nfse.DPS {
	t.Helper()
	id, err := nfse.ID("3550308", "11222333000181", 1, 42)
	if err != nil {
		t.Fatal(err)
	}
	if id != "DPS355030821122233300018100001000000000000042" {
		t.Errorf("Id: %s", id)
	}
	ind := 0
	return &nfse.DPS{InfDPS: nfse.InfDPS{
		ID: id, TpAmb: 2, DhEmi: nfse.Timestamp(time.Date(2026, 11, 8, 10, 0, 0, 0, time.FixedZone("-03", -3*3600))),
		VerAplic: "kang-fiscal", Serie: "1", NDPS: "42", DCompet: "2026-11-08", TpEmit: 1, FinNFSe: 0, CLocEmi: "3550308",
		Prest: nfse.Prest{CNPJ: "11222333000181", RegTrib: nfse.RegTrib{OpSimpNac: 1, RegEspTrib: 0}},
		Toma: &nfse.Pessoa{CNPJ: "99888777000166", XNome: "CLIENTE LTDA", Email: "financeiro@cliente.com.br",
			End: &nfse.Endereco{EndNac: nfse.EndNac{CMun: "3550308", CEP: "01310100"}, XLgr: "AV PAULISTA", Nro: "1000", XBairro: "BELA VISTA"}},
		Serv: nfse.Serv{LocPrest: nfse.LocPrest{CLocPrestacao: "3550308"},
			CServ: nfse.CServ{CTribNac: "010701", XDescServ: "Suporte tecnico em informatica"}},
		Valores: nfse.Valores{VServPrest: nfse.VServPrest{VServ: d("1500.00")},
			Trib: nfse.Trib{TribMun: nfse.TribMun{TribISSQN: 1, TpRetISSQN: 1}, TotTrib: nfse.TotTrib{IndTotTrib: &ind}}},
	}}
}

// Prestador do regime normal, com a classificação do IBS/CBS (o Sistema Nacional calcula).
func TestDPSRegimeNormalComIBSCBSPassaNoXSD(t *testing.T) {
	dps := base(t)
	dps.InfDPS.IBSCBS = &nfse.IBSCBS{IndDest: 0, Valores: nfse.IBSCBSValores{Trib: nfse.IBSCBSTrib{CST: "000", CClassTrib: "000001"}}}
	validate(t, signed(t, dps))
}

// Simples Nacional (ME/EPP), tomador pessoa física sem endereço, percentual do Simples.
func TestDPSSimplesNacionalPassaNoXSD(t *testing.T) {
	dps := base(t)
	ap := 1
	dps.InfDPS.Prest.RegTrib = nfse.RegTrib{OpSimpNac: 3, RegApTribSN: &ap, RegEspTrib: 0}
	dps.InfDPS.Toma = &nfse.Pessoa{CPF: "52998224725", XNome: "FULANO DE TAL"}
	p := d("6.00")
	dps.InfDPS.Valores.Trib.TotTrib = nfse.TotTrib{PTotTribSN: &p}
	validate(t, signed(t, dps))
}

func TestDPSRecusaIdForaDoLeiaute(t *testing.T) {
	if _, err := nfse.ID("355030", "11222333000181", 1, 1); err == nil {
		t.Error("município de 6 dígitos")
	}
	if _, err := nfse.ID("3550308", "11222333000181", 1, 0); err == nil {
		t.Error("número zero")
	}
}
