package nfce_test

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/accesskey"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"

	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfce"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
)

func TestTodasAsUFsTemURL(t *testing.T) {
	for _, uf := range []string{"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA", "MG", "MS", "MT", "PA",
		"PB", "PE", "PI", "PR", "RJ", "RN", "RO", "RR", "RS", "SC", "SE", "SP", "TO"} {
		for _, amb := range []int{1, 2} {
			if _, _, err := nfce.URLs(uf, amb); err != nil {
				t.Errorf("%s tpAmb %d: %v", uf, amb, err)
			}
		}
	}
}

// Uma NFC-e de consumidor não identificado, com o QR Code v3, passa no XSD oficial — o
// padrão do qrCode no schema é o da NT 2025.001.
func TestNFCeComQRCodeV3PassaNoXSD(t *testing.T) {
	d := func(s string) decimal.Fixed { v, _ := decimal.ParseAuto(s); return v }
	items := []nfe.Det{{NItem: 1, Prod: nfe.Prod{CProd: "1", CEAN: "SEM GTIN",
		XProd: "NOTA FISCAL EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL", NCM: "21011110", CFOP: "5102",
		UCom: "UN", QCom: d("2.0000"), VUnCom: d("6.5000000000"), VProd: d("13.00"), CEANTrib: "SEM GTIN", UTrib: "UN",
		QTrib: d("2.0000"), VUnTrib: d("6.5000000000"), IndTot: 1},
		Imposto: nfe.Imposto{ICMS: &nfe.ICMS{ICMSSN102: &nfe.ICMSSN102{CSOSN: "102"}},
			PIS:    &nfe.PIS{PISOutr: &nfe.PISOutr{CST: "49", VBC: d("0.00"), PPIS: d("0.0000"), VPIS: d("0.00")}},
			COFINS: &nfe.COFINS{COFINSOutr: &nfe.COFINSOutr{CST: "49", VBC: d("0.00"), PCOFINS: d("0.0000"), VCOFINS: d("0.00")}}}}}
	total, _ := nfe.ComputeTotals(items, false)
	ide, id := nfetest.Ide(t, 65, 7)
	supl, err := nfce.Supplement("SP", id[3:], 2)
	if err != nil {
		t.Fatal(err)
	}
	if supl.QRCode != "https://www.homologacao.nfce.fazenda.sp.gov.br/qrcode?p="+id[3:]+"|3|2" {
		t.Errorf("QR Code: %s", supl.QRCode)
	}
	n := &nfe.NFe{InfNFe: nfe.InfNFe{ID: id, Ide: ide, Emit: nfetest.Emit(1), Det: items, Total: total,
		Transp: nfe.Transp{ModFrete: 9}, Pag: nfe.Pag{DetPag: []nfe.DetPag{{TPag: "01", VPag: d("20.00")}}, VTroco: d("7.00").Ptr()}},
		InfNFeSupl: supl}
	nfetest.MustValidateXSD(t, nfetest.Sign(t, n))
}

func TestNFCeOfflinePassaNoXSDEAssinaturaConfere(t *testing.T) {
	d := func(s string) decimal.Fixed { v, _ := decimal.ParseAuto(s); return v }
	items := []nfe.Det{{NItem: 1, Prod: nfe.Prod{CProd: "1", CEAN: "SEM GTIN",
		XProd: "NOTA FISCAL EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL", NCM: "21011110", CFOP: "5102",
		UCom: "UN", QCom: d("3.0000"), VUnCom: d("6.5000000000"), VProd: d("19.50"), CEANTrib: "SEM GTIN", UTrib: "UN",
		QTrib: d("3.0000"), VUnTrib: d("6.5000000000"), IndTot: 1},
		Imposto: nfe.Imposto{ICMS: &nfe.ICMS{ICMSSN102: &nfe.ICMSSN102{CSOSN: "102"}}}}}
	total, _ := nfe.ComputeTotals(items, false)
	ide, _ := nfetest.Ide(t, 65, 8)
	ide.TpEmis, ide.DhCont, ide.XJust = 9, ide.DhEmi, "Falha de comunicacao com a SEFAZ"
	key, _ := accesskey.Build(accesskey.Parts{UFCode: 35, Issued: time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC),
		Document: nfetest.CNPJ, Model: 65, Series: 1, Number: 8, EmissionType: 9, Code: 12345678})
	k := key.String()
	ide.CNF, ide.CDV = k[35:43], int(k[43]-'0')

	pfx, _ := certificate.TestPFX("CAFE EXEMPLO LTDA", nfetest.CNPJ, "s", time.Now().Add(time.Hour))
	cert, _ := certificate.Load(pfx, "s")
	supl, err := nfce.SupplementOffline("SP", nfce.Offline{AccessKey: k, TpAmb: 2, Day: 8, VNF: "19.50"}, cert)
	if err != nil {
		t.Fatal(err)
	}
	// A assinatura confere com a chave pública do certificado.
	p := supl.QRCode[strings.Index(supl.QRCode, "?p=")+3:]
	parts := strings.Split(p, "|")
	if len(parts) != 8 || parts[5] != "" || parts[6] != "" {
		t.Fatalf("parâmetros: %v", parts)
	}
	sig, _ := base64.StdEncoding.DecodeString(parts[7])
	sum := sha1.Sum([]byte(strings.Join(parts[:7], "|")))
	if err := rsa.VerifyPKCS1v15(cert.Leaf().PublicKey.(*rsa.PublicKey), crypto.SHA1, sum[:], sig); err != nil {
		t.Errorf("assinatura do QR Code: %v", err)
	}
	n := &nfe.NFe{InfNFe: nfe.InfNFe{ID: "NFe" + k, Ide: ide, Emit: nfetest.Emit(1), Det: items, Total: total,
		Transp: nfe.Transp{ModFrete: 9}, Pag: nfe.Pag{DetPag: []nfe.DetPag{{TPag: "01", VPag: d("19.50")}}}},
		InfNFeSupl: supl}
	nfetest.MustValidateXSD(t, nfetest.Sign(t, n))
}
