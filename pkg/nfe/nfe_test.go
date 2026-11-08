package nfe_test

import (
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
)

func d(s string, scale uint8) nfe.Dec { return mustDec(s, scale) }
func p(s string, scale uint8) *nfe.Dec {
	v := mustDec(s, scale)
	return &v
}

func mustDec(s string, scale uint8) decimal.Fixed {
	v, err := decimal.Parse(s, scale)
	if err != nil {
		panic(err)
	}
	return v
}

// ibsTeste2026 é o IBS/CBS do ano de teste: CBS 0,9% e IBS 0,1% (UF), destacados e
// compensáveis com PIS/COFINS. CST 000 / cClassTrib 000001 = tributação integral.
func ibsTeste2026(base string) *nfe.IBSCBS {
	vBC := d(base, 2)
	vCBS, _ := vBC.Percent(d("0.9000", 4), 2, decimal.HalfUp)
	vIBSUF, _ := vBC.Percent(d("0.1000", 4), 2, decimal.HalfUp)
	return &nfe.IBSCBS{
		CST: "000", CClassTrib: "000001",
		GIBSCBS: &nfe.GIBSCBS{
			VBC:     vBC,
			GIBSUF:  nfe.GIBSUF{PIBSUF: d("0.1000", 4), VIBSUF: vIBSUF},
			GIBSMun: nfe.GIBSMun{PIBSMun: d("0.0000", 4), VIBSMun: d("0.00", 2)},
			VIBS:    vIBSUF,
			GCBS:    nfe.GCBS{PCBS: d("0.9000", 4), VCBS: vCBS},
		},
	}
}

func notaRegimeNormal(t *testing.T) *nfe.NFe {
	t.Helper()
	items := []nfe.Det{
		{
			NItem: 1,
			Prod: nfe.Prod{CProd: "CAFE-500", CEAN: "SEM GTIN", XProd: "CAFE TORRADO EM GRAOS 500G", NCM: "09012100",
				CFOP: "5102", UCom: "UN", QCom: d("3.0000", 4), VUnCom: d("33.3300000000", 10), VProd: d("99.99", 2),
				CEANTrib: "SEM GTIN", UTrib: "UN", QTrib: d("3.0000", 4), VUnTrib: d("33.3300000000", 10), IndTot: 1},
			Imposto: nfe.Imposto{
				ICMS:   &nfe.ICMS{ICMS00: &nfe.ICMS00{Orig: 0, CST: "00", ModBC: 3, VBC: d("99.99", 2), PICMS: d("18.00", 2), VICMS: d("18.00", 2)}},
				PIS:    &nfe.PIS{PISAliq: &nfe.PISAliq{CST: "01", VBC: d("99.99", 2), PPIS: d("1.6500", 4), VPIS: d("1.65", 2)}},
				COFINS: &nfe.COFINS{COFINSAliq: &nfe.COFINSAliq{CST: "01", VBC: d("99.99", 2), PCOFINS: d("7.6000", 4), VCOFINS: d("7.60", 2)}},
				IBSCBS: ibsTeste2026("99.99"),
			},
		},
		{
			NItem: 2,
			Prod: nfe.Prod{CProd: "LIVRO-1", CEAN: "SEM GTIN", XProd: "LIVRO DE RECEITAS", NCM: "49019900",
				CFOP: "5102", UCom: "UN", QCom: d("1.0000", 4), VUnCom: d("50.0000000000", 10), VProd: d("50.00", 2),
				CEANTrib: "SEM GTIN", UTrib: "UN", QTrib: d("1.0000", 4), VUnTrib: d("50.0000000000", 10),
				VDesc: p("5.00", 2), IndTot: 1},
			Imposto: nfe.Imposto{
				ICMS:   &nfe.ICMS{ICMS40: &nfe.ICMS40{Orig: 0, CST: "40"}},
				PIS:    &nfe.PIS{PISNT: &nfe.PISNT{CST: "06"}},
				COFINS: &nfe.COFINS{COFINSNT: &nfe.COFINSNT{CST: "06"}},
				IBSCBS: ibsTeste2026("45.00"),
			},
		},
	}
	total, err := nfe.ComputeTotals(items, false)
	if err != nil {
		t.Fatal(err)
	}
	ide, id := nfetest.Ide(t, 55, 123)
	return &nfe.NFe{InfNFe: nfe.InfNFe{
		ID:   id,
		Ide:  ide,
		Emit: nfetest.Emit(3),
		Dest: &nfe.Dest{CNPJ: "99888777000166", XNome: "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL",
			EnderDest: &nfe.Endereco{XLgr: "AV PAULISTA", Nro: "1000", XBairro: "BELA VISTA", CMun: "3550308",
				XMun: "SAO PAULO", UF: "SP", CEP: "01310100"},
			IndIEDest: 9},
		Det:    items,
		Total:  total,
		Transp: nfe.Transp{ModFrete: 9},
		Pag:    nfe.Pag{DetPag: []nfe.DetPag{{TPag: "17", VPag: total.ICMSTot.VNF}}},
	}}
}

func runXSD(t *testing.T, doc []byte) ([]byte, error) { return nfetest.ValidateXSD(t, doc) }

// A validação só vale se recusa o errado: NCM de 7 dígitos e CST do IBS com 2 precisam
// cair no schema.
func TestXSDRecusaNotaErrada(t *testing.T) {
	for name, mutate := range map[string]func(*nfe.NFe){
		"NCM com 7 dígitos":  func(n *nfe.NFe) { n.InfNFe.Det[0].Prod.NCM = "0901210" },
		"CST do IBS curto":   func(n *nfe.NFe) { n.InfNFe.Det[0].Imposto.IBSCBS.CST = "00" },
		"cClassTrib curto":   func(n *nfe.NFe) { n.InfNFe.Det[0].Imposto.IBSCBS.CClassTrib = "1" },
		"alíquota sem casas": func(n *nfe.NFe) { n.InfNFe.Det[0].Imposto.ICMS.ICMS00.PICMS = d("18.123456", 6) },
	} {
		t.Run(name, func(t *testing.T) {
			n := notaRegimeNormal(t)
			mutate(n)
			if _, err := runXSD(t, nfetest.Sign(t, n)); err == nil {
				t.Error("o XSD aceitou uma nota errada")
			}
		})
	}
}

func TestNotaRegimeNormalComReforma(t *testing.T) {
	n := notaRegimeNormal(t)
	tot := n.InfNFe.Total

	// Tributos atuais: somados por item, e o vNF pela fórmula do manual.
	if tot.ICMSTot.VProd.String() != "149.99" || tot.ICMSTot.VDesc.String() != "5.00" ||
		tot.ICMSTot.VNF.String() != "144.99" || tot.ICMSTot.VICMS.String() != "18.00" ||
		tot.ICMSTot.VPIS.String() != "1.65" || tot.ICMSTot.VCOFINS.String() != "7.60" {
		t.Errorf("ICMSTot: %+v", tot.ICMSTot)
	}
	// Reforma: CBS 0,9% e IBS 0,1%, arredondados item a item e só então somados
	// (Constituição, Art. 5.3). CBS: 0,89991 → 0,90 e 0,405 → 0,41 = 1,31. Aplicar a alíquota
	// sobre a soma das bases (144,99 × 0,9% = 1,30) daria um centavo a menos — e a SEFAZ,
	// que soma os itens, rejeitaria.
	g := tot.IBSCBSTot
	if g == nil || g.VBCIBSCBS.String() != "144.99" || g.GCBS.VCBS.String() != "1.31" || g.GIBS.VIBS.String() != "0.15" {
		t.Errorf("IBSCBSTot: %+v %+v %+v", g, g.GCBS, g.GIBS)
	}
	// Em 2026 IBS e CBS não somam no vNF.
	if tot.VNFTot != nil {
		t.Error("vNFTot preenchido sem pedir")
	}

	nfetest.MustValidateXSD(t, nfetest.Sign(t, n))
}

func TestNFCeSimplesSemReforma(t *testing.T) {
	items := []nfe.Det{{
		NItem: 1,
		Prod: nfe.Prod{CProd: "1", CEAN: "SEM GTIN", XProd: "CAFE COADO", NCM: "21011110", CFOP: "5102",
			UCom: "UN", QCom: d("2.0000", 4), VUnCom: d("6.5000000000", 10), VProd: d("13.00", 2),
			CEANTrib: "SEM GTIN", UTrib: "UN", QTrib: d("2.0000", 4), VUnTrib: d("6.5000000000", 10), IndTot: 1},
		Imposto: nfe.Imposto{
			ICMS:   &nfe.ICMS{ICMSSN102: &nfe.ICMSSN102{Orig: 0, CSOSN: "102"}},
			PIS:    &nfe.PIS{PISOutr: &nfe.PISOutr{CST: "49", VBC: d("0.00", 2), PPIS: d("0.0000", 4), VPIS: d("0.00", 2)}},
			COFINS: &nfe.COFINS{COFINSOutr: &nfe.COFINSOutr{CST: "49", VBC: d("0.00", 2), PCOFINS: d("0.0000", 4), VCOFINS: d("0.00", 2)}},
		},
	}}
	total, err := nfe.ComputeTotals(items, false)
	if err != nil {
		t.Fatal(err)
	}
	if total.IBSCBSTot != nil || total.ICMSTot.VNF.String() != "13.00" {
		t.Fatalf("total: %+v", total)
	}
	ide, id := nfetest.Ide(t, 65, 7)
	n := &nfe.NFe{InfNFe: nfe.InfNFe{
		ID: id, Ide: ide, Emit: nfetest.Emit(1),
		Det: items, Total: total, Transp: nfe.Transp{ModFrete: 9},
		Pag: nfe.Pag{DetPag: []nfe.DetPag{{TPag: "01", VPag: d("20.00", 2)}}, VTroco: p("7.00", 2)},
	}}
	nfetest.MustValidateXSD(t, nfetest.Sign(t, n))
}

func TestVNFTotQuandoPedido(t *testing.T) {
	n := notaRegimeNormal(t)
	total, _ := nfe.ComputeTotals(n.InfNFe.Det, true)
	// 144,99 + 0,15 (IBS) + 1,31 (CBS)
	if total.VNFTot == nil || total.VNFTot.String() != "146.45" {
		t.Errorf("vNFTot: %v", total.VNFTot)
	}
}
