package danfe_test

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	zxqr "github.com/makiuchi-d/gozxing/qrcode"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/alfredokang/kang-fiscal/pkg/danfe"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfce"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
)

func nfceSigned(t *testing.T) ([]byte, string) {
	t.Helper()
	d := func(s string) decimal.Fixed { v, _ := decimal.ParseAuto(s); return v }
	items := []nfe.Det{{NItem: 1, Prod: nfe.Prod{CProd: "CAFE-COADO", CEAN: "SEM GTIN", XProd: "NOTA FISCAL EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL",
		NCM: "21011110", CFOP: "5102", UCom: "UN", QCom: d("3.0000"), VUnCom: d("6.5000000000"), VProd: d("19.50"),
		CEANTrib: "SEM GTIN", UTrib: "UN", QTrib: d("3.0000"), VUnTrib: d("6.5000000000"), IndTot: 1},
		Imposto: nfe.Imposto{ICMS: &nfe.ICMS{ICMSSN102: &nfe.ICMSSN102{CSOSN: "102"}}}}}
	total, _ := nfe.ComputeTotals(items, false)
	ide, id := nfetest.Ide(t, 65, 42)
	supl, _ := nfce.Supplement("SP", id[3:], 2)
	n := &nfe.NFe{InfNFe: nfe.InfNFe{ID: id, Ide: ide, Emit: nfetest.Emit(1), Det: items, Total: total,
		Transp: nfe.Transp{ModFrete: 9}, Pag: nfe.Pag{DetPag: []nfe.DetPag{{TPag: "01", VPag: d("20.00")}}, VTroco: d("0.50").Ptr()}},
		InfNFeSupl: supl}
	return nfetest.Sign(t, n), supl.QRCode
}

func TestDANFCE(t *testing.T) {
	doc, _ := nfceSigned(t)
	pdfBytes, err := danfe.Generate(doc, danfe.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if text := pdfToText(t, pdfBytes); text != "" {
		for _, want := range []string{"DANFE NFC-e", "CONSUMIDOR NÃO IDENTIFICADO", "Dinheiro", "Troco R$", "Consulte pela Chave de Acesso"} {
			if !strings.Contains(text, want) {
				t.Errorf("o DANFCE não mostra %q", want)
			}
		}
	}
}

// O QR Code tem de ser lido de volta por um decodificador independente (ZXing), com o
// conteúdo exato que vai no XML — é o que o celular do consumidor vai fazer.
func TestQRCodeLidoPorDecodificadorIndependente(t *testing.T) {
	_, content := nfceSigned(t)
	q, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		t.Fatal(err)
	}
	bitmap := q.Bitmap() // com a zona de silêncio, como um leitor espera
	scale := 4
	img := image.NewGray(image.Rect(0, 0, len(bitmap)*scale, len(bitmap)*scale))
	for y, row := range bitmap {
		for x, dark := range row {
			c := color.Gray{Y: 255}
			if dark {
				c = color.Gray{Y: 0}
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.SetGray(x*scale+dx, y*scale+dy, c)
				}
			}
		}
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	res, err := zxqr.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		t.Fatalf("o QR Code não foi lido: %v", err)
	}
	if res.GetText() != content {
		t.Errorf("lido %q, esperava %q", res.GetText(), content)
	}
}
