package tax_test

import (
	"errors"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
)

func d(s string, scale uint8) decimal.Fixed {
	v, err := decimal.Parse(s, scale)
	if err != nil {
		panic(err)
	}
	return v
}

// 2026, ano de teste: CBS 0,9% e IBS 0,1% (UF), município zero.
var rates2026 = tax.Rates{CBS: d("0.9", 4), IBSUF: d("0.1", 4), IBSMun: d("0", 4), Version: "2026-teste"}

func vendaRegimeNormal() tax.Profile {
	return tax.Profile{
		CFOPInternal: "5102", CFOPInterstate: "6102",
		ICMS:                   tax.ICMSRule{Code: "00", Rate: d("18", 2)},
		PIS:                    tax.PISCOFINSRule{CST: "01", Rate: d("1.65", 4)},
		COFINS:                 tax.PISCOFINSRule{CST: "01", Rate: d("7.6", 4)},
		IBSCBS:                 &tax.IBSCBSRule{CST: "000", ClassTrib: "000001"},
		ExcludeICMSFromPISBase: true,
	}
}

func item(qty, price string) tax.Item {
	return tax.Item{Quantity: d(qty, 4), UnitPrice: d(price, 10)}
}

func TestRegimeNormalTributosAtuaisEReforma(t *testing.T) {
	res, err := tax.Compute(item("3", "33.33"), vendaRegimeNormal(), rates2026, tax.Context{})
	if err != nil {
		t.Fatal(err)
	}
	im := res.Imposto
	// vProd = 3 × 33,33 = 99,99; ICMS 18% = 17,9982 → 18,00
	// PIS/COFINS sem o ICMS na base: 81,99 × 1,65% = 1,3528 → 1,35; × 7,6% = 6,2312 → 6,23
	// IBS/CBS sem ICMS, PIS e COFINS: 99,99 − 18,00 − 1,35 − 6,23 = 74,41
	//   CBS 0,9% = 0,66969 → 0,67; IBS UF 0,1% = 0,07441 → 0,07
	checks := map[string][2]string{
		"vProd":   {res.VProd.String(), "99.99"},
		"vICMS":   {im.ICMS.ICMS00.VICMS.String(), "18.00"},
		"vBC PIS": {im.PIS.PISAliq.VBC.String(), "81.99"},
		"vPIS":    {im.PIS.PISAliq.VPIS.String(), "1.35"},
		"vCOFINS": {im.COFINS.COFINSAliq.VCOFINS.String(), "6.23"},
		"vBC IBS": {im.IBSCBS.GIBSCBS.VBC.String(), "74.41"},
		"vCBS":    {im.IBSCBS.GIBSCBS.GCBS.VCBS.String(), "0.67"},
		"vIBSUF":  {im.IBSCBS.GIBSCBS.GIBSUF.VIBSUF.String(), "0.07"},
		"vIBS":    {im.IBSCBS.GIBSCBS.VIBS.String(), "0.07"},
		"pCBS":    {im.IBSCBS.GIBSCBS.GCBS.PCBS.String(), "0.9000"},
		"CFOP":    {res.CFOP, "5102"},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %s, esperava %s", name, c[0], c[1])
		}
	}

	inter, _ := tax.Compute(item("3", "33.33"), vendaRegimeNormal(), rates2026, tax.Context{Interstate: true})
	if inter.CFOP != "6102" {
		t.Errorf("CFOP interestadual: %s", inter.CFOP)
	}
}

// Regime diferenciado da LC 214/2025: redução de 60% na alíquota (cesta estendida).
func TestReducaoDeAliquota(t *testing.T) {
	p := vendaRegimeNormal()
	p.ICMS = tax.ICMSRule{Code: "40"}
	p.PIS = tax.PISCOFINSRule{CST: "06"}
	p.COFINS = tax.PISCOFINSRule{CST: "06"}
	p.IBSCBS = &tax.IBSCBSRule{CST: "200", ClassTrib: "200003", RateReduction: d("60", 2)}
	res, err := tax.Compute(item("1", "1000"), p, rates2026, tax.Context{})
	if err != nil {
		t.Fatal(err)
	}
	g := res.Imposto.IBSCBS.GIBSCBS
	// CBS efetiva 0,9 × 40% = 0,36% → 3,60; IBS UF 0,1 × 40% = 0,04% → 0,40
	if g.GCBS.GRed == nil || g.GCBS.GRed.PAliqEfet.String() != "0.3600" || g.GCBS.VCBS.String() != "3.60" ||
		g.GIBSUF.VIBSUF.String() != "0.40" || g.GCBS.PCBS.String() != "0.9000" {
		t.Errorf("redução: CBS %+v %+v, IBS UF %+v", g.GCBS, g.GCBS.GRed, g.GIBSUF)
	}
}

func TestSimplesNacional(t *testing.T) {
	p := tax.Profile{
		CFOPInternal: "5102", CFOPInterstate: "6102",
		ICMS:   tax.ICMSRule{Code: "102"},
		PIS:    tax.PISCOFINSRule{CST: "49"},
		COFINS: tax.PISCOFINSRule{CST: "49"},
	}
	res, err := tax.Compute(item("2", "6.5"), p, rates2026, tax.Context{Simples: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Imposto.ICMS.ICMSSN102 == nil || res.Imposto.IBSCBS != nil || res.Imposto.PIS.PISOutr.VPIS.String() != "0.00" {
		t.Errorf("Simples: %+v", res.Imposto)
	}
	// Simples com crédito de ICMS (101): 13,00 × 2,5% = 0,325 → 0,33
	p.ICMS = tax.ICMSRule{Code: "101", SimplesCreditRate: d("2.5", 2)}
	res, _ = tax.Compute(item("2", "6.5"), p, rates2026, tax.Context{Simples: true})
	if res.Imposto.ICMS.ICMSSN101.VCredICMSSN.String() != "0.33" {
		t.Errorf("crédito do Simples: %s", res.Imposto.ICMS.ICMSSN101.VCredICMSSN)
	}
}

func TestRecusas(t *testing.T) {
	p := vendaRegimeNormal()
	if _, err := tax.Compute(item("1", "10"), p, rates2026, tax.Context{Simples: true}); !errors.Is(err, tax.ErrRegime) {
		t.Errorf("CST normal no Simples: %v", err)
	}
	p.ICMS.Code = "10" // ST com retenção ainda não é calculada aqui
	if _, err := tax.Compute(item("1", "10"), p, rates2026, tax.Context{}); !errors.Is(err, tax.ErrUnsupportedCST) {
		t.Errorf("CST 10: %v", err)
	}
	it := item("1", "10")
	it.Discount = d("11", 2)
	if _, err := tax.Compute(it, vendaRegimeNormal(), rates2026, tax.Context{}); !errors.Is(err, tax.ErrInvalidItem) {
		t.Errorf("desconto maior que o item: %v", err)
	}
	if _, err := tax.Compute(item("0", "10"), vendaRegimeNormal(), rates2026, tax.Context{}); !errors.Is(err, tax.ErrInvalidItem) {
		t.Errorf("quantidade zero: %v", err)
	}
}

// A nota inteira montada pelo motor passa no XSD oficial.
func TestNotaDoMotorPassaNoXSD(t *testing.T) {
	var items []nfe.Det
	for i, it := range []tax.Item{item("3", "33.33"), item("1.5", "19.9")} {
		res, err := tax.Compute(it, vendaRegimeNormal(), rates2026, tax.Context{})
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, nfe.Det{NItem: i + 1, Prod: nfe.Prod{
			CProd: "P" + string(rune('1'+i)), CEAN: "SEM GTIN", XProd: "CAFE", NCM: "09012100", CFOP: res.CFOP,
			UCom: "UN", QCom: it.Quantity, VUnCom: it.UnitPrice, VProd: res.VProd,
			CEANTrib: "SEM GTIN", UTrib: "UN", QTrib: it.Quantity, VUnTrib: it.UnitPrice, IndTot: 1,
		}, Imposto: res.Imposto})
	}
	total, err := nfe.ComputeTotals(items, false)
	if err != nil {
		t.Fatal(err)
	}
	ide, id := nfetest.Ide(t, 55, 99)
	n := &nfe.NFe{InfNFe: nfe.InfNFe{ID: id, Ide: ide, Emit: nfetest.Emit(3),
		Dest: &nfe.Dest{CNPJ: "99888777000166", XNome: "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL", IndIEDest: 9},
		Det:  items, Total: total, Transp: nfe.Transp{ModFrete: 9},
		Pag: nfe.Pag{DetPag: []nfe.DetPag{{TPag: "17", VPag: total.ICMSTot.VNF}}}}}
	nfetest.MustValidateXSD(t, nfetest.Sign(t, n))
}
