package danfe

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/alfredokang/kang-fiscal/pkg/danfe/internal/pdf"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
)

// Bobina de 80 mm: 72 mm úteis com a margem que as impressoras térmicas exigem.
const (
	rollWidth  = 80.0
	rollMargin = 4.0
	rollInner  = rollWidth - 2*rollMargin
)

// renderNFCe gera o DANFCE (MOC DANFE NFC-e): cupom em bobina, com o QR Code que leva o
// consumidor à consulta da nota. A altura da página acompanha a quantidade de itens.
func (r *renderer) renderNFCe(supl *nfe.InfNFeSupl) ([]byte, error) {
	n := r.n
	height := 128.0 + float64(len(n.Det))*6 + float64(len(n.Pag.DetPag))*3.5
	r.page = r.doc.AddPage(rollWidth, height)
	pg := r.page
	x, y := rollMargin, 4.0
	center := func(text string, st pdf.Style) {
		for _, line := range pdf.Wrap(text, st.Font, st.Size, rollInner) {
			pg.TextCenter(x, rollInner, y, line, st)
			y += st.Size/2.83 + 0.9
		}
	}
	rule := func() {
		y += 0.8
		for dx := x; dx < x+rollInner; dx += 1.6 {
			pg.Line(dx, y, dx+0.8, y, 0.15)
		}
		y += 1.4
	}
	bold := pdf.Style{Font: pdf.Bold, Size: 7}
	text := pdf.Style{Size: 6.5}
	tiny := pdf.Style{Size: 5.5}

	e := n.Emit
	a := e.EnderEmit
	center(e.XNome, bold)
	center("CNPJ: "+document(e.CNPJ, e.CPF)+"  IE: "+e.IE, tiny)
	center(fmt.Sprintf("%s, %s %s - %s - %s/%s", a.XLgr, a.Nro, a.XCpl, a.XBairro, a.XMun, a.UF), tiny)
	rule()
	center("DANFE NFC-e - Documento Auxiliar da Nota Fiscal de Consumidor Eletrônica", bold)
	if n.Ide.TpEmis == 9 {
		center("EMITIDA EM CONTINGÊNCIA", bold)
	}
	if n.Ide.TpAmb == 2 {
		center("EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL", bold)
	}
	rule()

	// Itens: código e descrição numa linha, quantidade × unitário = total na de baixo.
	pg.Text(x, y, "CÓDIGO  DESCRIÇÃO", tiny)
	pg.TextRight(x, rollInner, y, "QTD UN x VL UNIT = VL TOTAL", tiny)
	y += 2.6
	for _, it := range n.Det {
		p := it.Prod
		pg.Text(x, y, pdf.Truncate(p.CProd+"  "+p.XProd, pdf.Regular, 6.5, rollInner), text)
		y += 2.6
		pg.TextRight(x, rollInner, y, fmt.Sprintf("%s %s x %s = %s", trimZeros(p.QCom.String()), p.UCom,
			trimZeros(p.VUnCom.String()), money(p.VProd.String())), text)
		y += 3.2
	}
	rule()

	t := n.Total.ICMSTot
	line := func(label, value string, st pdf.Style) {
		pg.Text(x, y, label, st)
		pg.TextRight(x, rollInner, y, value, st)
		y += 3
	}
	line("Qtde. total de itens", fmt.Sprint(len(n.Det)), text)
	line("Valor total R$", money(t.VProd.String()), text)
	if !t.VDesc.IsZero() {
		line("Descontos R$", money(t.VDesc.String()), text)
	}
	if !t.VOutro.IsZero() || !t.VFrete.IsZero() {
		other, _ := t.VOutro.Add(t.VFrete)
		line("Acréscimos R$", money(other.String()), text)
	}
	line("Valor a pagar R$", money(t.VNF.String()), bold)
	line("FORMA DE PAGAMENTO", "VALOR PAGO R$", tiny)
	for _, p := range n.Pag.DetPag {
		line(paymentName(p.TPag), money(p.VPag.String()), text)
	}
	if n.Pag.VTroco != nil {
		line("Troco R$", money(n.Pag.VTroco.String()), text)
	}
	rule()

	key := strings.TrimPrefix(n.ID, "NFe")
	urlChave := ""
	if supl != nil {
		urlChave = supl.URLChave
	}
	center("Consulte pela Chave de Acesso em", tiny)
	center(urlChave, tiny)
	center(groupKey(key), pdf.Style{Font: pdf.Bold, Size: 6})
	rule()
	if d := n.Dest; d != nil && (d.CPF != "" || d.CNPJ != "") {
		center("CONSUMIDOR - "+document(d.CNPJ, d.CPF), text)
		if d.XNome != "" {
			center(d.XNome, tiny)
		}
	} else {
		center("CONSUMIDOR NÃO IDENTIFICADO", text)
	}
	rule()
	center(fmt.Sprintf("NFC-e nº %d  Série %d  %s", n.Ide.NNF, n.Ide.Serie, dateTime(n.Ide.DhEmi)), bold)
	if r.protocol != "" {
		center("Protocolo de autorização: "+r.protocol, text)
		center("Data de autorização: "+dateTime(r.received), text)
	} else if n.Ide.TpEmis == 9 {
		center("Pendente de autorização", text)
	} else {
		center("NOTA NÃO AUTORIZADA", bold)
	}

	if supl != nil && supl.QRCode != "" {
		q, err := qrcode.New(supl.QRCode, qrcode.Medium)
		if err != nil {
			return nil, fmt.Errorf("danfe: QR Code: %w", err)
		}
		q.DisableBorder = true
		size := 32.0
		y += 2
		pg.QRCode(x+(rollInner-size)/2, y, size, pdf.QRMatrix(q.Bitmap()))
		y += size + 3
	}
	if v := t.VTotTrib; v != nil {
		center("Tributos totais incidentes (Lei Federal 12.741/2012): R$ "+money(v.String()), tiny)
	}
	if g := n.Total.IBSCBSTot; g != nil && g.GIBS != nil && g.GCBS != nil {
		center(fmt.Sprintf("Reforma tributária: IBS R$ %s  CBS R$ %s", money(g.GIBS.VIBS.String()), money(g.GCBS.VCBS.String())), tiny)
	}
	if a := n.InfAdic; a != nil && a.InfCpl != "" {
		center(a.InfCpl, tiny)
	}
	if r.opts.Canceled {
		center("NOTA CANCELADA", pdf.Style{Font: pdf.Bold, Size: 10})
	}
	center("Gerado por Kang Fiscal", pdf.Style{Size: 4.5, Gray: 0.4})
	return r.doc.Bytes()
}

func paymentName(tPag string) string {
	switch tPag {
	case "01":
		return "Dinheiro"
	case "02":
		return "Cheque"
	case "03":
		return "Cartão de Crédito"
	case "04":
		return "Cartão de Débito"
	case "05":
		return "Crédito Loja"
	case "10":
		return "Vale Alimentação"
	case "11":
		return "Vale Refeição"
	case "15":
		return "Boleto"
	case "16":
		return "Depósito"
	case "17":
		return "PIX"
	case "90":
		return "Sem pagamento"
	}
	return "Outros"
}
