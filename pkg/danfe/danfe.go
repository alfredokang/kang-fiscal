// Package danfe gera o DANFE (Documento Auxiliar da NF-e) em PDF, sem dependência: só a
// biblioteca padrão e as fontes base-14, que todo leitor de PDF já tem.
//
// O layout segue os blocos do Manual de Orientação do Contribuinte (Anexo II): canhoto,
// cabeçalho com o código de barras da chave, protocolo, destinatário, cálculo do imposto,
// transportador, produtos e dados adicionais. Os valores de IBS e CBS vão nos dados
// adicionais — o DANFE é auxiliar, e o leiaute oficial do quadro de impostos não mudou.
//
// O DANFE não substitui o XML: a validade jurídica é do arquivo autorizado. O DANFE só
// acompanha a mercadoria e permite consultar a nota pela chave.
package danfe

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/danfe/internal/pdf"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
)

// ErrNotNFe: o XML não é uma NF-e nem um nfeProc.
var ErrNotNFe = errors.New("danfe: o XML não é uma NF-e (NFe ou nfeProc)")

// Options ajusta o documento.
type Options struct {
	// Canceled carimba "CANCELADA" — para reimprimir uma nota que teve o cancelamento
	// homologado.
	Canceled bool
}

type proc struct {
	XMLName xml.Name `xml:"nfeProc"`
	NFe     nfe.NFe  `xml:"NFe"`
	Prot    struct {
		Info struct {
			NProt    string `xml:"nProt"`
			DhRecbto string `xml:"dhRecbto"`
		} `xml:"infProt"`
	} `xml:"protNFe"`
}

// icmsAny lê o grupo de ICMS de qualquer CST, inclusive os que o Kang Fiscal não emite
// (ICMSSN202, ICMS10...): o DANFE imprime nota de qualquer emissor.
type icmsAny struct {
	Groups []struct {
		XMLName xml.Name
		Orig    string `xml:"orig"`
		CST     string `xml:"CST"`
		CSOSN   string `xml:"CSOSN"`
		VBC     string `xml:"vBC"`
		VICMS   string `xml:"vICMS"`
		PICMS   string `xml:"pICMS"`
	} `xml:",any"`
}

type lenient struct {
	Det []struct {
		ICMS icmsAny `xml:"imposto>ICMS"`
	} `xml:"infNFe>det"`
}

// Generate gera o DANFE a partir do nfeProc (ou do XML assinado, sem protocolo).
func Generate(doc []byte, opts Options) ([]byte, error) {
	var p proc
	if err := xml.Unmarshal(doc, &p); err != nil || p.NFe.InfNFe.ID == "" {
		var n nfe.NFe
		if err := xml.Unmarshal(doc, &n); err != nil || n.InfNFe.ID == "" {
			return nil, ErrNotNFe
		}
		p.NFe = n
	}
	r := &renderer{n: &p.NFe.InfNFe, protocol: p.Prot.Info.NProt, received: p.Prot.Info.DhRecbto, opts: opts, doc: pdf.New()}
	var l struct {
		NFe  lenient `xml:"NFe"`
		Self lenient
	}
	_ = xml.Unmarshal(doc, &l)
	_ = xml.Unmarshal(doc, &l.Self)
	src := l.NFe
	if len(src.Det) == 0 {
		src = l.Self
	}
	for _, d := range src.Det {
		r.icms = append(r.icms, d.ICMS)
	}
	return r.render()
}

const (
	margin = 5.0
	width  = pdf.A4Width - 2*margin
	// rowsFirst e rowsNext são quantos itens cabem na primeira página (com todos os
	// quadros) e nas seguintes (só cabeçalho e produtos).
	rowHeight = 4.2
)

var (
	label  = pdf.Style{Size: 5}
	value  = pdf.Style{Size: 7.5}
	strong = pdf.Style{Font: pdf.Bold, Size: 8}
	small  = pdf.Style{Size: 6}
)

type renderer struct {
	n        *nfe.InfNFe
	protocol string
	received string
	opts     Options
	doc      *pdf.PDF
	page     *pdf.Page
	pages    int
	icms     []icmsAny // por item, na ordem do det
	item     int
}

func (r *renderer) render() ([]byte, error) {
	items := r.n.Det
	first := true
	for len(items) > 0 || first {
		r.page = r.doc.AddPageA4()
		r.pages++
		y := margin
		if first {
			y = r.receipt(y)
		}
		y = r.header(y)
		if first {
			y = r.recipient(y)
			y = r.taxes(y)
			y = r.transport(y)
		}
		// O espaço dos itens é o que sobra na página, deixando os dados adicionais na
		// primeira (onde a SEFAZ e quem recebe procuram).
		bottom := pdf.A4Height - margin
		if first {
			bottom -= 34
		}
		y, items = r.products(y, bottom, items)
		if first {
			r.additional(pdf.A4Height - margin - 32)
		}
		r.stamp()
		first = false
	}
	// "Folha X/Y" só se conhece no fim; o cabeçalho escreve "Folha N" e a contagem total
	// vai no rodapé de cada página.
	return r.doc.Bytes()
}

// box desenha um campo: retângulo, rótulo pequeno em cima e valor embaixo.
func (r *renderer) box(x, y, w, h float64, title, text string, st pdf.Style) {
	r.page.Rect(x, y, w, h, 0.2)
	r.page.Text(x+0.8, y+0.6, title, label)
	r.page.Text(x+0.8, y+2.6, pdf.Truncate(text, st.Font, st.Size, w-1.6), st)
}

func (r *renderer) boxRight(x, y, w, h float64, title, text string) {
	r.page.Rect(x, y, w, h, 0.2)
	r.page.Text(x+0.8, y+0.6, title, label)
	r.page.TextRight(x, w-0.8, y+2.6, text, value)
}

func (r *renderer) receipt(y float64) float64 {
	h := 18.0
	e := r.n.Emit
	r.page.Rect(margin, y, width, h, 0.2)
	r.page.TextWrapped(margin+1, y+0.8, width-50, fmt.Sprintf("RECEBEMOS DE %s OS PRODUTOS E/OU SERVIÇOS CONSTANTES DA NOTA FISCAL ELETRÔNICA INDICADA AO LADO. EMISSÃO: %s VALOR TOTAL: R$ %s DESTINATÁRIO: %s",
		e.XNome, date(r.n.Ide.DhEmi), money(r.n.Total.ICMSTot.VNF.String()), destName(r.n.Dest)), pdf.Style{Size: 5.5}, 3)
	r.page.Line(margin, y+9, margin+width-45, y+9, 0.2)
	r.box(margin, y+9, 40, h-9, "DATA DE RECEBIMENTO", "", value)
	r.box(margin+40, y+9, width-85, h-9, "IDENTIFICAÇÃO E ASSINATURA DO RECEBEDOR", "", value)
	r.page.Rect(margin+width-45, y, 45, h, 0.2)
	r.page.TextCenter(margin+width-45, 45, y+4, "NF-e", strong)
	r.page.TextCenter(margin+width-45, 45, y+8, "Nº "+number(r.n.Ide.NNF), strong)
	r.page.TextCenter(margin+width-45, 45, y+12, fmt.Sprintf("SÉRIE %03d", r.n.Ide.Serie), strong)
	// Linha de corte do canhoto.
	for x := margin; x < margin+width; x += 2 {
		r.page.Line(x, y+h+1.5, x+1, y+h+1.5, 0.15)
	}
	return y + h + 3
}

func (r *renderer) header(y float64) float64 {
	h := 30.0
	e := r.n.Emit
	a := e.EnderEmit
	ew := 80.0
	r.page.Rect(margin, y, ew, h, 0.2)
	r.page.TextWrapped(margin+2, y+2, ew-4, e.XNome, pdf.Style{Font: pdf.Bold, Size: 9}, 2)
	addr := fmt.Sprintf("%s, %s %s\n%s - %s - %s/%s\nCEP %s  %s", a.XLgr, a.Nro, a.XCpl, a.XBairro, cep(a.CEP), a.XMun, a.UF, cep(a.CEP), phone(a.Fone))
	addr = strings.Replace(addr, " - "+cep(a.CEP)+" ", " ", 1)
	r.page.TextWrapped(margin+2, y+11, ew-4, addr, small, 5)

	dx := margin + ew
	dw := 32.0
	r.page.Rect(dx, y, dw, h, 0.2)
	r.page.TextCenter(dx, dw, y+2, "DANFE", pdf.Style{Font: pdf.Bold, Size: 11})
	r.page.TextWrapped(dx+2, y+7, dw-4, "Documento Auxiliar da Nota Fiscal Eletrônica", small, 3)
	r.page.Text(dx+2, y+15, "0 - ENTRADA", small)
	r.page.Text(dx+2, y+18, "1 - SAÍDA", small)
	r.page.Rect(dx+dw-8, y+14.5, 6, 5, 0.2)
	r.page.TextCenter(dx+dw-8, 6, y+15.3, fmt.Sprint(r.n.Ide.TpNF), strong)
	r.page.Text(dx+2, y+21.5, "Nº "+number(r.n.Ide.NNF), pdf.Style{Font: pdf.Bold, Size: 7})
	r.page.Text(dx+2, y+24.5, fmt.Sprintf("SÉRIE %03d", r.n.Ide.Serie), pdf.Style{Font: pdf.Bold, Size: 7})
	r.page.Text(dx+2, y+27, fmt.Sprintf("FOLHA %d", r.pages), small)

	kx := dx + dw
	kw := width - ew - dw
	key := strings.TrimPrefix(r.n.ID, "NFe")
	r.page.Rect(kx, y, kw, h, 0.2)
	if err := r.page.Barcode128C(kx+3, y+2, kw-6, 11, key); err != nil {
		r.page.Text(kx+3, y+5, "chave inválida para código de barras", small)
	}
	r.page.Text(kx+1, y+14.5, "CHAVE DE ACESSO", label)
	r.page.TextCenter(kx, kw, y+16.5, groupKey(key), pdf.Style{Font: pdf.Bold, Size: 7.2})
	r.page.TextWrapped(kx+2, y+21, kw-4, "Consulta de autenticidade no portal nacional da NF-e www.nfe.fazenda.gov.br/portal ou no site da SEFAZ autorizadora", small, 3)
	y += h

	prot := r.protocol
	if prot != "" {
		prot += "  " + dateTime(r.received)
	} else {
		prot = "SEM PROTOCOLO — NOTA NÃO AUTORIZADA"
	}
	r.box(margin, y, ew+dw, 7, "NATUREZA DA OPERAÇÃO", r.n.Ide.NatOp, value)
	r.box(kx, y, kw, 7, "PROTOCOLO DE AUTORIZAÇÃO DE USO", prot, value)
	y += 7
	third := width / 3
	r.box(margin, y, third, 7, "INSCRIÇÃO ESTADUAL", e.IE, value)
	r.box(margin+third, y, third, 7, "INSCRIÇÃO ESTADUAL DO SUBST. TRIBUTÁRIO", e.IEST, value)
	r.box(margin+2*third, y, third, 7, "CNPJ / CPF", document(e.CNPJ, e.CPF), value)
	return y + 8
}

func (r *renderer) section(y float64, title string) float64 {
	r.page.Text(margin, y, title, pdf.Style{Font: pdf.Bold, Size: 6})
	return y + 2.6
}

func (r *renderer) recipient(y float64) float64 {
	y = r.section(y, "DESTINATÁRIO / REMETENTE")
	d := r.n.Dest
	if d == nil {
		d = &nfe.Dest{XNome: "CONSUMIDOR NÃO IDENTIFICADO"}
	}
	a := nfe.Endereco{}
	if d.EnderDest != nil {
		a = *d.EnderDest
	}
	dw := 32.0
	r.box(margin, y, width-2*dw-35, 7, "NOME / RAZÃO SOCIAL", d.XNome, value)
	r.box(margin+width-2*dw-35, y, 35, 7, "CNPJ / CPF", document(d.CNPJ, d.CPF), value)
	r.box(margin+width-2*dw, y, 2*dw, 7, "DATA DA EMISSÃO", date(r.n.Ide.DhEmi), value)
	y += 7
	r.box(margin, y, width-2*dw-55, 7, "ENDEREÇO", strings.TrimSpace(a.XLgr+", "+a.Nro+" "+a.XCpl), value)
	r.box(margin+width-2*dw-55, y, 35, 7, "BAIRRO / DISTRITO", a.XBairro, value)
	r.box(margin+width-2*dw-20, y, 20, 7, "CEP", cep(a.CEP), value)
	r.box(margin+width-2*dw, y, 2*dw, 7, "DATA DA SAÍDA/ENTRADA", date(r.n.Ide.DhSaiEnt), value)
	y += 7
	r.box(margin, y, width-2*dw-55, 7, "MUNICÍPIO", a.XMun, value)
	r.box(margin+width-2*dw-55, y, 35, 7, "FONE / FAX", phone(a.Fone), value)
	r.box(margin+width-2*dw-20, y, 20, 7, "UF", a.UF, value)
	r.box(margin+width-2*dw, y, 2*dw, 7, "INSCRIÇÃO ESTADUAL", d.IE, value)
	return y + 8
}

func (r *renderer) taxes(y float64) float64 {
	y = r.section(y, "CÁLCULO DO IMPOSTO")
	t := r.n.Total.ICMSTot
	w := width / 6
	row1 := [][2]string{{"BASE DE CÁLC. DO ICMS", t.VBC.String()}, {"VALOR DO ICMS", t.VICMS.String()},
		{"BASE DE CÁLC. ICMS S.T.", t.VBCST.String()}, {"VALOR DO ICMS SUBST.", t.VST.String()},
		{"V. TOTAL TRIBUTOS", opt(t.VTotTrib)}, {"V. TOTAL PRODUTOS", t.VProd.String()}}
	row2 := [][2]string{{"VALOR DO FRETE", t.VFrete.String()}, {"VALOR DO SEGURO", t.VSeg.String()},
		{"DESCONTO", t.VDesc.String()}, {"OUTRAS DESPESAS", t.VOutro.String()},
		{"VALOR TOTAL IPI", t.VIPI.String()}, {"V. TOTAL DA NOTA", t.VNF.String()}}
	for _, row := range [][][2]string{row1, row2} {
		for i, c := range row {
			r.boxRight(margin+float64(i)*w, y, w, 7, c[0], money(c[1]))
		}
		y += 7
	}
	return y + 1
}

func (r *renderer) transport(y float64) float64 {
	y = r.section(y, "TRANSPORTADOR / VOLUMES TRANSPORTADOS")
	tr := r.n.Transp
	name, doc, uf := "", "", ""
	if t := tr.Transporta; t != nil {
		name, doc, uf = t.XNome, document(t.CNPJ, t.CPF), t.UF
	}
	r.box(margin, y, width-95, 7, "RAZÃO SOCIAL", name, value)
	r.box(margin+width-95, y, 45, 7, "FRETE POR CONTA", freight(tr.ModFrete), value)
	r.box(margin+width-50, y, 15, 7, "UF", uf, value)
	r.box(margin+width-35, y, 35, 7, "CNPJ / CPF", doc, value)
	return y + 8
}

type column struct {
	title string
	w     float64
	right bool
}

var columns = []column{
	{"CÓDIGO", 16, false}, {"DESCRIÇÃO DO PRODUTO / SERVIÇO", 58, false}, {"NCM/SH", 13, false},
	{"CST", 8, false}, {"CFOP", 8, false}, {"UN", 7, false}, {"QUANT.", 13, true}, {"V.UNIT.", 14, true},
	{"V.TOTAL", 14, true}, {"BC ICMS", 13, true}, {"V.ICMS", 11, true}, {"V.IPI", 10, true}, {"%ICMS", 8, true}, {"%IPI", 7, true},
}

func (r *renderer) products(y, bottom float64, items []nfe.Det) (float64, []nfe.Det) {
	y = r.section(y, "DADOS DOS PRODUTOS / SERVIÇOS")
	x := margin
	for _, c := range columns {
		r.page.Rect(x, y, c.w, 4, 0.2)
		r.page.TextCenter(x, c.w, y+1, c.title, label)
		x += c.w
	}
	y += 4
	top := y
	for len(items) > 0 && y+rowHeight <= bottom {
		it := items[0]
		var icms *icmsAny
		if r.item < len(r.icms) {
			icms = &r.icms[r.item]
		}
		cells := itemCells(it, icms)
		x = margin
		lines := pdf.Wrap(cells[1], pdf.Regular, 6, columns[1].w-1)
		if len(lines) > 3 {
			lines = lines[:3]
		}
		// 2,7 mm por linha de texto de 6 pt, mais respiro em cima e embaixo.
		h := float64(len(lines))*2.7 + 1.2
		if h < rowHeight {
			h = rowHeight
		}
		if y+h > bottom {
			break
		}
		for i, c := range columns {
			switch {
			case i == 1:
				for j, l := range lines {
					r.page.Text(x+0.5, y+0.8+float64(j)*2.7, l, small)
				}
			case c.right:
				r.page.TextRight(x, c.w-0.5, y+0.8, cells[i], small)
			default:
				r.page.Text(x+0.5, y+0.8, pdf.Truncate(cells[i], pdf.Regular, 6, c.w-1), small)
			}
			x += c.w
		}
		y += h
		r.page.Line(margin, y, margin+width, y, 0.1)
		items = items[1:]
		r.item++
	}
	r.page.Rect(margin, top, width, y-top, 0.2)
	x = margin
	for _, c := range columns[:len(columns)-1] {
		x += c.w
		r.page.Line(x, top, x, y, 0.1)
	}
	if len(items) > 0 {
		r.page.Text(margin, y+1, "CONTINUA NA PRÓXIMA FOLHA", small)
	}
	return y + 2, items
}

func (r *renderer) additional(y float64) {
	r.section(y, "DADOS ADICIONAIS")
	y += 2.6
	var b strings.Builder
	if a := r.n.InfAdic; a != nil {
		for _, s := range []string{a.InfAdFisco, a.InfCpl} {
			if s != "" {
				b.WriteString(s + "\n")
			}
		}
	}
	if g := r.n.Total.IBSCBSTot; g != nil {
		fmt.Fprintf(&b, "Reforma tributária (LC 214/2025) — base IBS/CBS R$ %s", money(g.VBCIBSCBS.String()))
		if g.GIBS != nil {
			fmt.Fprintf(&b, "; IBS R$ %s", money(g.GIBS.VIBS.String()))
		}
		if g.GCBS != nil {
			fmt.Fprintf(&b, "; CBS R$ %s", money(g.GCBS.VCBS.String()))
		}
		b.WriteString(".\n")
	}
	if r.n.Ide.TpAmb == 2 {
		b.WriteString("NOTA EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO — SEM VALOR FISCAL.\n")
	}
	iw := width * 0.68
	r.page.Rect(margin, y, iw, 28, 0.2)
	r.page.Text(margin+0.8, y+0.6, "INFORMAÇÕES COMPLEMENTARES", label)
	r.page.TextWrapped(margin+1, y+3, iw-2, strings.TrimSpace(b.String()), small, 9)
	r.page.Rect(margin+iw, y, width-iw, 28, 0.2)
	r.page.Text(margin+iw+0.8, y+0.6, "RESERVADO AO FISCO", label)
	r.page.Text(margin, y+29, "Gerado por Kang Fiscal — github.com/alfredokang/kang-fiscal", pdf.Style{Size: 4.5, Gray: 0.4})
}

// stamp carimba homologação e cancelamento em diagonal leve, sem cobrir os dados.
func (r *renderer) stamp() {
	text := ""
	switch {
	case r.opts.Canceled:
		text = "CANCELADA"
	case r.n.Ide.TpAmb == 2:
		text = "SEM VALOR FISCAL"
	case r.protocol == "":
		text = "NÃO AUTORIZADA"
	}
	if text != "" {
		r.page.TextCenter(margin, width, 150, text, pdf.Style{Font: pdf.Bold, Size: 40, Gray: 0.8})
	}
}

func itemCells(it nfe.Det, any *icmsAny) []string {
	p := it.Prod
	cst, bc, vicms, picms := icmsCells(it.Imposto.ICMS)
	if cst == "" && any != nil && len(any.Groups) > 0 {
		g := any.Groups[0]
		cst, bc, vicms, picms = g.Orig+g.CST+g.CSOSN, g.VBC, g.VICMS, g.PICMS
	}
	vipi, pipi := "", ""
	if ipi := it.Imposto.IPI; ipi != nil && ipi.IPITrib != nil {
		vipi, pipi = ipi.IPITrib.VIPI.String(), ipi.IPITrib.PIPI.String()
	}
	return []string{p.CProd, p.XProd, p.NCM, cst, p.CFOP, p.UCom, trimZeros(p.QCom.String()), trimZeros(p.VUnCom.String()),
		money(p.VProd.String()), money(bc), money(vicms), money(vipi), trimZeros(picms), trimZeros(pipi)}
}

// icmsCells devolve CST/CSOSN (com a origem na frente, como o DANFE mostra), base, valor e
// alíquota do ICMS do item.
func icmsCells(i *nfe.ICMS) (cst, bc, v, p string) {
	if i == nil {
		return "", "", "", ""
	}
	o := func(orig int, code string) string { return fmt.Sprint(orig) + code }
	switch {
	case i.ICMS00 != nil:
		g := i.ICMS00
		return o(g.Orig, g.CST), g.VBC.String(), g.VICMS.String(), g.PICMS.String()
	case i.ICMS20 != nil:
		g := i.ICMS20
		return o(g.Orig, g.CST), g.VBC.String(), g.VICMS.String(), g.PICMS.String()
	case i.ICMS40 != nil:
		return o(i.ICMS40.Orig, i.ICMS40.CST), "", "", ""
	case i.ICMS60 != nil:
		return o(i.ICMS60.Orig, i.ICMS60.CST), "", "", ""
	case i.ICMS90 != nil:
		return o(i.ICMS90.Orig, i.ICMS90.CST), opt(i.ICMS90.VBC), opt(i.ICMS90.VICMS), opt(i.ICMS90.PICMS)
	case i.ICMSSN101 != nil:
		return o(i.ICMSSN101.Orig, i.ICMSSN101.CSOSN), "", "", ""
	case i.ICMSSN102 != nil:
		return o(i.ICMSSN102.Orig, i.ICMSSN102.CSOSN), "", "", ""
	case i.ICMSSN500 != nil:
		return o(i.ICMSSN500.Orig, i.ICMSSN500.CSOSN), "", "", ""
	case i.ICMSSN900 != nil:
		return o(i.ICMSSN900.Orig, i.ICMSSN900.CSOSN), opt(i.ICMSSN900.VBC), opt(i.ICMSSN900.VICMS), opt(i.ICMSSN900.PICMS)
	}
	return "", "", "", ""
}

func opt(d *nfe.Dec) string {
	if d == nil {
		return ""
	}
	return d.String()
}

// money formata 1234.5 como 1.234,50 — o DANFE é lido por gente, não por máquina.
func money(s string) string {
	if s == "" {
		return ""
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac, _ := strings.Cut(s, ".")
	frac = (frac + "00")[:2]
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	out := b.String() + "," + frac
	if neg {
		out = "-" + out
	}
	return out
}

// trimZeros mostra quantidade e valor unitário sem zeros inúteis, mas com 2 casas no mínimo.
func trimZeros(s string) string {
	if s == "" {
		return ""
	}
	intPart, frac, ok := strings.Cut(s, ".")
	if !ok {
		return intPart + ",00"
	}
	frac = strings.TrimRight(frac, "0")
	for len(frac) < 2 {
		frac += "0"
	}
	return intPart + "," + frac
}

func number(n int) string {
	s := fmt.Sprintf("%09d", n)
	return s[0:3] + "." + s[3:6] + "." + s[6:9]
}

func groupKey(k string) string {
	var parts []string
	for i := 0; i < len(k); i += 4 {
		end := min(i+4, len(k))
		parts = append(parts, k[i:end])
	}
	return strings.Join(parts, " ")
}

func document(cnpj, cpf string) string {
	switch {
	case len(cnpj) == 14:
		return cnpj[0:2] + "." + cnpj[2:5] + "." + cnpj[5:8] + "/" + cnpj[8:12] + "-" + cnpj[12:14]
	case len(cpf) == 11:
		return cpf[0:3] + "." + cpf[3:6] + "." + cpf[6:9] + "-" + cpf[9:11]
	}
	return cnpj + cpf
}

func destName(d *nfe.Dest) string {
	if d == nil {
		return "CONSUMIDOR NÃO IDENTIFICADO"
	}
	return d.XNome
}

func cep(c string) string {
	if len(c) == 8 {
		return c[:5] + "-" + c[5:]
	}
	return c
}

func phone(f string) string {
	if f == "" {
		return ""
	}
	return "Fone: " + f
}

func date(s string) string {
	t, err := time.Parse("2006-01-02T15:04:05-07:00", s)
	if err != nil {
		return ""
	}
	return t.Format("02/01/2006")
}

func dateTime(s string) string {
	t, err := time.Parse("2006-01-02T15:04:05-07:00", s)
	if err != nil {
		return s
	}
	return t.Format("02/01/2006 15:04:05")
}

func freight(mod int) string {
	switch mod {
	case 0:
		return "0 - EMITENTE"
	case 1:
		return "1 - DESTINATÁRIO"
	case 2:
		return "2 - TERCEIROS"
	case 3:
		return "3 - PRÓPRIO REMETENTE"
	case 4:
		return "4 - PRÓPRIO DESTINATÁRIO"
	case 9:
		return "9 - SEM FRETE"
	}
	return fmt.Sprint(mod)
}
