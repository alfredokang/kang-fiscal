package danfe

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/alfredokang/kang-fiscal/pkg/danfe/internal/pdf"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/ibge"
)

// ErrNotNFSe: o XML não é uma NFS-e do padrão nacional.
var ErrNotNFSe = errors.New("danfe: o XML não é uma NFS-e do padrão nacional (NFSe)")

// NFSeQueryURL é a consulta pública da NFS-e, para onde o QR Code do DANFSe leva.
const NFSeQueryURL = "https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave="

// O XML da NFS-e lido com folga: só o que vai no papel, tudo texto. A NFS-e é gerada
// pelo Sistema Nacional e muda de versão sem aviso; um campo novo não pode impedir a
// impressão.
type sNFSe struct {
	XMLName xml.Name `xml:"NFSe"`
	Inf     sInfNFSe `xml:"infNFSe"`
}

type sInfNFSe struct {
	ID            string   `xml:"Id,attr"`
	XLocEmi       string   `xml:"xLocEmi"`
	XLocPrestacao string   `xml:"xLocPrestacao"`
	NNFSe         string   `xml:"nNFSe"`
	CLocIncid     string   `xml:"cLocIncid"`
	XLocIncid     string   `xml:"xLocIncid"`
	XTribNac      string   `xml:"xTribNac"`
	XTribMun      string   `xml:"xTribMun"`
	XNBS          string   `xml:"xNBS"`
	AmbGer        string   `xml:"ambGer"`
	CStat         string   `xml:"cStat"`
	DhProc        string   `xml:"dhProc"`
	Emit          sParty   `xml:"emit"`
	Valores       sValNFSe `xml:"valores"`
	XOutInf       string   `xml:"xOutInf"`
	IBSCBS        *sIBSNF  `xml:"IBSCBS"`
	DPS           struct {
		Inf sInfDPS `xml:"infDPS"`
	} `xml:"DPS"`
}

type sAddr struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun"`
	UF      string `xml:"UF"`
	CEP     string `xml:"CEP"`
	EndNac  struct {
		CMun string `xml:"cMun"`
		CEP  string `xml:"CEP"`
	} `xml:"endNac"`
	EndExt struct {
		CPais   string `xml:"cPais"`
		XCidade string `xml:"xCidade"`
	} `xml:"endExt"`
}

type sParty struct {
	CNPJ     string `xml:"CNPJ"`
	CPF      string `xml:"CPF"`
	NIF      string `xml:"NIF"`
	IM       string `xml:"IM"`
	XNome    string `xml:"xNome"`
	Fone     string `xml:"fone"`
	Email    string `xml:"email"`
	EnderNac *sAddr `xml:"enderNac"`
	End      *sAddr `xml:"end"`
	RegTrib  struct {
		OpSimpNac   string `xml:"opSimpNac"`
		RegApTribSN string `xml:"regApTribSN"`
		RegEspTrib  string `xml:"regEspTrib"`
	} `xml:"regTrib"`
}

type sValNFSe struct {
	VCalcDR    string `xml:"vCalcDR"`
	VCalcBM    string `xml:"vCalcBM"`
	VBC        string `xml:"vBC"`
	PAliqAplic string `xml:"pAliqAplic"`
	VISSQN     string `xml:"vISSQN"`
	VTotalRet  string `xml:"vTotalRet"`
	VLiq       string `xml:"vLiq"`
}

type sIBSNF struct {
	CLocalidadeIncid string `xml:"cLocalidadeIncid"`
	XLocalidadeIncid string `xml:"xLocalidadeIncid"`
	Valores          struct {
		VBC string `xml:"vBC"`
		UF  struct {
			PIBSUF      string `xml:"pIBSUF"`
			PRedAliqUF  string `xml:"pRedAliqUF"`
			PAliqEfetUF string `xml:"pAliqEfetUF"`
		} `xml:"uf"`
		Mun struct {
			PIBSMun      string `xml:"pIBSMun"`
			PRedAliqMun  string `xml:"pRedAliqMun"`
			PAliqEfetMun string `xml:"pAliqEfetMun"`
		} `xml:"mun"`
		Fed struct {
			PCBS         string `xml:"pCBS"`
			PRedAliqCBS  string `xml:"pRedAliqCBS"`
			PAliqEfetCBS string `xml:"pAliqEfetCBS"`
		} `xml:"fed"`
	} `xml:"valores"`
	TotCIBS struct {
		VTotNF string `xml:"vTotNF"`
		GIBS   struct {
			VIBSTot   string `xml:"vIBSTot"`
			GIBSUFTot struct {
				VIBSUF string `xml:"vIBSUF"`
			} `xml:"gIBSUFTot"`
			GIBSMunTot struct {
				VIBSMun string `xml:"vIBSMun"`
			} `xml:"gIBSMunTot"`
		} `xml:"gIBS"`
		GCBS struct {
			VCBS string `xml:"vCBS"`
		} `xml:"gCBS"`
	} `xml:"totCIBS"`
}

type sClass struct {
	CST        string `xml:"CST"`
	CClassTrib string `xml:"cClassTrib"`
}

type sInfDPS struct {
	TpAmb   string `xml:"tpAmb"`
	DhEmi   string `xml:"dhEmi"`
	Serie   string `xml:"serie"`
	NDPS    string `xml:"nDPS"`
	DCompet string `xml:"dCompet"`
	TpEmit  string `xml:"tpEmit"`
	FinNFSe string `xml:"finNFSe"`
	Subst   *struct {
		ChSubstda string `xml:"chSubstda"`
	} `xml:"subst"`
	Prest  sParty  `xml:"prest"`
	Toma   *sParty `xml:"toma"`
	Interm *sParty `xml:"interm"`
	Serv   struct {
		LocPrest struct {
			CLocPrestacao  string `xml:"cLocPrestacao"`
			CPaisPrestacao string `xml:"cPaisPrestacao"`
		} `xml:"locPrest"`
		CServ struct {
			CTribNac  string `xml:"cTribNac"`
			CTribMun  string `xml:"cTribMun"`
			XDescServ string `xml:"xDescServ"`
			CNBS      string `xml:"cNBS"`
		} `xml:"cServ"`
		InfoCompl struct {
			XInfComp string `xml:"xInfComp"`
		} `xml:"infoCompl"`
	} `xml:"serv"`
	Valores struct {
		VServPrest struct {
			VServ string `xml:"vServ"`
		} `xml:"vServPrest"`
		VDescCondIncond struct {
			VDescIncond string `xml:"vDescIncond"`
			VDescCond   string `xml:"vDescCond"`
		} `xml:"vDescCondIncond"`
		Trib struct {
			TribMun struct {
				TribISSQN   string `xml:"tribISSQN"`
				TpImunidade string `xml:"tpImunidade"`
				TpRetISSQN  string `xml:"tpRetISSQN"`
				PAliq       string `xml:"pAliq"`
				ExigSusp    struct {
					TpSusp    string `xml:"tpSusp"`
					NProcesso string `xml:"nProcesso"`
				} `xml:"exigSusp"`
			} `xml:"tribMun"`
			TribFed struct {
				Piscofins struct {
					CST            string `xml:"CST"`
					VPis           string `xml:"vPis"`
					VCofins        string `xml:"vCofins"`
					TpRetPisCofins string `xml:"tpRetPisCofins"`
				} `xml:"piscofins"`
				VRetCP   string `xml:"vRetCP"`
				VRetIRRF string `xml:"vRetIRRF"`
				VRetCSLL string `xml:"vRetCSLL"`
			} `xml:"tribFed"`
			TotTrib struct {
				VTotTrib struct {
					VTotTribFed string `xml:"vTotTribFed"`
					VTotTribEst string `xml:"vTotTribEst"`
					VTotTribMun string `xml:"vTotTribMun"`
				} `xml:"vTotTrib"`
				PTotTribSN string `xml:"pTotTribSN"`
			} `xml:"totTrib"`
		} `xml:"trib"`
	} `xml:"valores"`
	IBSCBS *struct {
		FinNFSe string `xml:"finNFSe"`
		CIndOp  string `xml:"cIndOp"`
		IndDest string `xml:"indDest"`
		Valores struct {
			Trib struct {
				sClass
				G *sClass `xml:"gIBSCBS"`
			} `xml:"trib"`
		} `xml:"valores"`
	} `xml:"IBSCBS"`
}

// Textos dos códigos (leiaute da NFS-e e NT 008/2026).
var (
	nfseTpEmit  = map[string]string{"1": "Prestador", "2": "Tomador", "3": "Intermediário"}
	nfseAmbGer  = map[string]string{"1": "Sistema Próprio do Município", "2": "Sefin Nacional NFS-e"}
	nfseTpAmb   = map[string]string{"1": "Produção", "2": "Homologação"}
	nfseCStat   = map[string]string{"100": "NFS-e Gerada", "101": "NFS-e de Substituição Gerada", "102": "NFS-e de Decisão Judicial ou Administrativa", "103": "NFS-e Avulsa", "107": "NFS-e MEI"}
	nfseFin     = map[string]string{"0": "NFS-e regular", "1": "NFS-e de crédito", "2": "NFS-e de débito"}
	nfseTribISS = map[string]string{"1": "Operação Tributável", "2": "Imunidade", "3": "Exportação de serviço", "4": "Não Incidência"}
	nfseRetISS  = map[string]string{"1": "Não Retido", "2": "Retido pelo Tomador", "3": "Retido pelo Intermediário"}
	nfseSusp    = map[string]string{"1": "Suspensa por Decisão Judicial", "2": "Suspensa por Processo Administrativo"}
	nfseSimples = map[string]string{"1": "Não Optante", "2": "Optante - MEI", "3": "Optante - ME/EPP"}
	nfseRegApSN = map[string]string{
		"1": "Tributos federais e municipal apurados pelo Simples Nacional",
		"2": "Tributos federais pelo SN e ISSQN pela NFS-e (legislação municipal)",
		"3": "Tributos federais e municipal pela NFS-e (legislações federal e municipal)",
	}
	nfseRegEsp = map[string]string{"0": "Nenhum", "1": "Ato Cooperado (Cooperativa)", "2": "Estimativa", "3": "Microempresa Municipal", "4": "Notário ou Registrador", "5": "Profissional Autônomo", "6": "Sociedade de Profissionais", "9": "Outros"}
	nfseImunid = map[string]string{"0": "Imunidade não especificada", "1": "Patrimônio, renda ou serviços (CF, art. 150, VI, a)", "2": "Templos de qualquer culto (CF, art. 150, VI, b)", "3": "Partidos, sindicatos, educação e assistência social (CF, art. 150, VI, c)", "4": "Livros, jornais, periódicos e papel (CF, art. 150, VI, d)", "5": "Fonogramas e videofonogramas nacionais (CF, art. 150, VI, e)"}
	nfseRetPCC = map[string]string{"0": "PIS/COFINS/CSLL Não Retidos", "1": "PIS/COFINS Retido", "2": "PIS/COFINS Não Retido", "3": "PIS/COFINS/CSLL Retidos", "4": "PIS/COFINS Retidos, CSLL Não Retido", "5": "PIS Retido, COFINS/CSLL Não Retido", "6": "COFINS Retido, PIS/CSLL Não Retido", "7": "PIS Não Retido, COFINS/CSLL Retidos", "8": "PIS/COFINS Não Retidos, CSLL Retido", "9": "COFINS Não Retido, PIS/CSLL Retidos"}
)

// GenerateNFSe gera o DANFSe (leiaute v2.0 da NT 008/2026) a partir do XML da NFS-e
// devolvido pelo Sistema Nacional. A API de PDF do governo foi descontinuada em 01/07/2026:
// o DANFSe é gerado aqui.
func GenerateNFSe(doc []byte, opts Options) ([]byte, error) {
	var n sNFSe
	if err := xml.Unmarshal(doc, &n); err != nil || n.Inf.ID == "" {
		return nil, ErrNotNFSe
	}
	r := &nfseRenderer{n: &n.Inf, d: &n.Inf.DPS.Inf, opts: opts, doc: pdf.New()}
	r.page = r.doc.AddPageA4()
	return r.render()
}

const (
	sRow   = 6.4 // altura de uma linha de campos
	sStrip = 3.6 // linhas de supressão ("não identificado")
	sColW  = width / 4
)

var (
	sLabel = pdf.Style{Font: pdf.Bold, Size: 5.5}
	sValue = pdf.Style{Size: 7}
	sTitle = pdf.Style{Font: pdf.Bold, Size: 7}
)

type nfseRenderer struct {
	n    *sInfNFSe
	d    *sInfDPS
	opts Options
	doc  *pdf.PDF
	page *pdf.Page
	y    float64
}

func (r *nfseRenderer) render() ([]byte, error) {
	pg := r.page
	r.y = margin
	pg.Rect(margin, margin, width, pdf.A4Height-2*margin, 0.353)
	r.header()
	if err := r.identification(); err != nil {
		return nil, err
	}
	r.provider()
	r.taker()
	r.strips()
	r.service()
	r.municipal()
	r.federal()
	r.ibscbs()
	r.totals()
	r.additional()
	pg.Text(margin+1, pdf.A4Height-margin-3, "Gerado por Kang Fiscal — github.com/alfredokang/kang-fiscal", pdf.Style{Size: 4.5, Gray: 0.4})
	if r.opts.Canceled {
		pg.TextCenter(margin, width, 140, "CANCELADA", pdf.Style{Font: pdf.Bold, Size: 60, Gray: 0.8})
	}
	return r.doc.Bytes()
}

// field escreve rótulo e valor numa célula da grade (col de 0 a 3, span colunas).
func (r *nfseRenderer) field(col, span int, label, val string) {
	x := margin + float64(col)*sColW
	w := float64(span)*sColW - 1.6
	r.page.Text(x+0.8, r.y+0.5, pdf.Truncate(label, sLabel.Font, sLabel.Size, w), sLabel)
	r.page.Text(x+0.8, r.y+2.9, pdf.Truncate(dash(val), sValue.Font, sValue.Size, w), sValue)
}

// title abre uma seção: o nome em negrito sobre fundo cinza, na primeira coluna.
func (r *nfseRenderer) title(text string) {
	r.page.FillRect(margin+0.2, r.y+0.2, sColW-0.4, sRow-0.4, 0.95)
	r.page.Text(margin+0.8, r.y+2, pdf.Truncate(text, sTitle.Font, sTitle.Size, sColW-1.6), sTitle)
}

func (r *nfseRenderer) rule() { r.page.Line(margin, r.y, margin+width, r.y, 0.176) }

func (r *nfseRenderer) next() { r.y += sRow }

func (r *nfseRenderer) strip(text string) {
	r.rule()
	r.page.TextCenter(margin, width, r.y+0.6, text, pdf.Style{Font: pdf.Bold, Size: 6.5})
	r.y += sStrip
}

func (r *nfseRenderer) header() {
	pg := r.page
	y := r.y
	pg.Text(margin+2, y+2.5, "NFS-e", pdf.Style{Font: pdf.Bold, Size: 20})
	pg.Text(margin+25, y+3.5, "Nota Fiscal de", pdf.Style{Size: 6.5, Gray: 0.35})
	pg.Text(margin+25, y+6.2, "Serviço Eletrônica", pdf.Style{Size: 6.5, Gray: 0.35})
	pg.TextCenter(margin+sColW, 2*sColW, y+2.5, "DANFSe v2.0", pdf.Style{Font: pdf.Bold, Size: 10})
	pg.TextCenter(margin+sColW, 2*sColW, y+6.5, "Documento Auxiliar da NFS-e", pdf.Style{Font: pdf.Bold, Size: 10})
	mun := r.n.XLocEmi
	if m, ok := ibge.Lookup(r.d.Prest.cityCode(r.n.Emit)); ok {
		mun = m.Name + " / " + m.UF
	}
	x := margin + 3*sColW
	pg.Text(x, y+2, "Município: "+dash(mun), pdf.Style{Size: 7.5})
	pg.Text(x, y+6, "Ambiente Gerador: "+dash(nfseAmbGer[r.n.AmbGer]), pdf.Style{Size: 5.5})
	pg.Text(x, y+8.5, "Tipo de Ambiente: "+dash(nfseTpAmb[r.d.TpAmb]), pdf.Style{Size: 5.5})
	r.y += 12
	if r.d.TpAmb == "2" {
		r.rule()
		r.page.TextCenter(margin, width, r.y+0.6, "NFS-e SEM VALIDADE JURÍDICA", pdf.Style{Font: pdf.Bold, Size: 7})
		r.y += sStrip
	}
	r.rule()
}

func (r *nfseRenderer) identification() error {
	key := strings.TrimPrefix(r.n.ID, "NFS")
	top := r.y
	r.field(0, 3, "CHAVE DE ACESSO DA NFS-e", key)
	r.next()
	r.field(0, 1, "NÚMERO DA NFS-e", r.n.NNFSe)
	r.field(1, 1, "COMPETÊNCIA DA NFS-e", isoDate(r.d.DCompet))
	r.field(2, 1, "DATA E HORA DA EMISSÃO DA NFS-e", dateTimeMin(r.n.DhProc))
	r.next()
	r.field(0, 1, "NÚMERO DA DPS", r.d.NDPS)
	r.field(1, 1, "SÉRIE DA DPS", r.d.Serie)
	r.field(2, 1, "DATA E HORA DA EMISSÃO DA DPS", dateTimeMin(r.d.DhEmi))
	r.next()
	fin := r.d.FinNFSe
	if fin == "" && r.d.IBSCBS != nil {
		fin = r.d.IBSCBS.FinNFSe
	}
	r.field(0, 1, "EMITENTE DA NFS-e", nfseTpEmit[r.d.TpEmit])
	r.field(1, 1, "SITUAÇÃO DA NFS-e", or(nfseCStat[r.n.CStat], r.n.CStat))
	r.field(2, 1, "FINALIDADE", or(nfseFin[fin], fin))
	r.next()

	// QR Code (NT 008: área útil mínima de 1,52 cm) e o texto de autenticidade.
	q, err := qrcode.New(NFSeQueryURL+key, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("danfe: QR Code: %w", err)
	}
	q.DisableBorder = true
	size := 17.0
	x := margin + 3*sColW
	r.page.QRCode(x+(sColW-size)/2, top+0.8, size, pdf.QRMatrix(q.Bitmap()))
	r.page.TextWrapped(x+1, top+size+1.6, sColW-2, "A autenticidade desta NFS-e pode ser verificada pela leitura deste código QR ou pela consulta da chave de acesso no portal nacional da NFS-e", pdf.Style{Size: 4.8}, 3)
	if bottom := top + size + 7.5; bottom > r.y {
		r.y = bottom
	}
	return nil
}

// cityCode é o município do prestador: no endereço da DPS ou no do emitente da NFS-e.
func (p sParty) cityCode(emit sParty) string {
	if a := p.addr(); a != nil && a.CMun != "" {
		return a.CMun
	}
	if a := emit.addr(); a != nil {
		return a.CMun
	}
	return ""
}

func (p sParty) addr() *sAddr {
	a := p.End
	if a == nil {
		a = p.EnderNac
	}
	if a == nil {
		return nil
	}
	out := *a
	if out.CMun == "" {
		out.CMun = a.EndNac.CMun
	}
	if out.CEP == "" {
		out.CEP = a.EndNac.CEP
	}
	return &out
}

// party escreve o bloco de prestador, tomador ou intermediário.
func (r *nfseRenderer) party(title string, p sParty) {
	r.rule()
	r.title(title)
	doc := document(p.CNPJ, p.CPF)
	if doc == "" {
		doc = p.NIF
	}
	r.field(1, 1, "CNPJ / CPF / NIF", doc)
	r.field(2, 1, "Indicador Municipal (Inscrição)", p.IM)
	r.field(3, 1, "Telefone", fone(p.Fone))
	r.next()
	a := p.addr()
	city, ibgeCEP, street := "", "", ""
	if a != nil {
		if m, ok := ibge.Lookup(a.CMun); ok {
			city = m.Name + " / " + m.UF
		} else if a.EndExt.XCidade != "" {
			city = a.EndExt.XCidade + " / " + a.EndExt.CPais
		}
		ibgeCEP = joinNonEmpty(" / ", a.CMun, cepNT(a.CEP))
		street = joinNonEmpty(", ", a.XLgr, a.Nro, a.XCpl, a.XBairro)
	}
	r.field(0, 2, "Nome / Nome Empresarial", p.XNome)
	r.field(2, 1, "Município / Sigla UF", city)
	r.field(3, 1, "Código IBGE / CEP", ibgeCEP)
	r.next()
	r.field(0, 2, "Endereço", street)
	r.field(2, 2, "E-mail", p.Email)
	r.next()
}

func (r *nfseRenderer) provider() {
	p := r.d.Prest
	// Quando o prestador é o emitente, a DPS traz só o CNPJ: nome, endereço e contato
	// vêm do emitente da NFS-e.
	e := r.n.Emit
	if p.XNome == "" {
		p.XNome = e.XNome
	}
	if p.End == nil {
		p.End = e.EnderNac
	}
	p.Fone, p.Email = or(p.Fone, e.Fone), or(p.Email, e.Email)
	if p.IM == "" {
		p.IM = e.IM
	}
	r.party("PRESTADOR / FORNECEDOR", p)
	rt := p.RegTrib
	r.field(0, 2, "Simples Nacional na Data de Competência", nfseSimples[rt.OpSimpNac])
	r.field(2, 2, "Regime de Apuração Tributária pelo SN", nfseRegApSN[rt.RegApTribSN])
	r.next()
}

func (r *nfseRenderer) taker() {
	if r.d.Toma == nil {
		r.strip("TOMADOR / ADQUIRENTE DA OPERAÇÃO NÃO IDENTIFICADO NA NFS-e")
		return
	}
	r.party("TOMADOR / ADQUIRENTE", *r.d.Toma)
}

func (r *nfseRenderer) strips() {
	if g := r.d.IBSCBS; g == nil || g.IndDest == "" || g.IndDest == "0" {
		r.strip("O DESTINATÁRIO É O PRÓPRIO TOMADOR / ADQUIRENTE DA OPERAÇÃO")
	} else {
		r.strip("DESTINATÁRIO DA OPERAÇÃO INFORMADO NO XML DA NFS-e")
	}
	if r.d.Interm == nil {
		r.strip("INTERMEDIÁRIO DA OPERAÇÃO NÃO IDENTIFICADO NA NFS-e")
	} else {
		r.party("INTERMEDIÁRIO", *r.d.Interm)
	}
}

func (r *nfseRenderer) service() {
	r.rule()
	r.title("SERVIÇO PRESTADO")
	s := r.d.Serv
	code := s.CServ.CTribNac
	if len(code) == 6 {
		code = code[0:2] + "." + code[2:4] + "." + code[4:6]
	}
	r.field(1, 1, "Código de Tributação Nacional / Municipal", joinNonEmpty(" / ", code, s.CServ.CTribMun))
	r.field(2, 1, "Código da NBS", s.CServ.CNBS)
	place := r.n.XLocPrestacao
	if m, ok := ibge.Lookup(s.LocPrest.CLocPrestacao); ok {
		place = m.Name + " / " + m.UF
	} else if s.LocPrest.CPaisPrestacao != "" {
		place = s.LocPrest.CPaisPrestacao
	}
	r.field(3, 1, "Local da Prestação / Sigla UF / País", place)
	r.next()
	r.field(0, 4, "Tributação Nacional", joinNonEmpty(" | ", r.n.XTribNac, r.n.XTribMun))
	r.next()
	r.page.Text(margin+0.8, r.y+0.5, "Descrição do Serviço", sLabel)
	h := r.page.TextWrapped(margin+0.8, r.y+2.9, width-1.6, dash(s.CServ.XDescServ), sValue, 8)
	r.y += 3.2 + h
}

func (r *nfseRenderer) municipal() {
	t := r.d.Valores.Trib.TribMun
	r.rule()
	r.title("TRIBUTAÇÃO MUNICIPAL (ISSQN)")
	r.field(1, 1, "Tipo de Tributação do ISSQN", nfseTribISS[t.TribISSQN])
	incid := r.n.XLocIncid
	if m, ok := ibge.Lookup(r.n.CLocIncid); ok {
		incid = m.Name + " / " + m.UF + " / BR"
	}
	r.field(2, 2, "Município / Sigla UF / País da Incidência do ISSQN", incid)
	r.next()
	r.field(0, 1, "Regime Especial de Tributação do ISSQN", nfseRegEsp[or(r.d.Prest.RegTrib.RegEspTrib, "0")])
	r.field(1, 1, "Tipo de Imunidade do ISSQN", nfseImunid[t.TpImunidade])
	r.field(2, 1, "Suspensão da Exigibilidade do ISSQN", nfseSusp[t.ExigSusp.TpSusp])
	r.field(3, 1, "Número Processo Suspensão", t.ExigSusp.NProcesso)
	r.next()
	v := r.n.Valores
	r.field(0, 1, "Cálculo do Benefício Municipal", brl(v.VCalcBM))
	r.field(1, 1, "Total Deduções / Reduções", brl(v.VCalcDR))
	r.field(2, 1, "Desconto Incondicionado", brl(r.d.Valores.VDescCondIncond.VDescIncond))
	r.next()
	r.field(0, 1, "BC ISSQN", brl(v.VBC))
	r.field(1, 1, "Alíquota Aplicada", pct(or(v.PAliqAplic, t.PAliq)))
	r.field(2, 1, "Retenção do ISSQN", nfseRetISS[t.TpRetISSQN])
	r.field(3, 1, "ISSQN Apurado", brl(v.VISSQN))
	r.next()
}

func (r *nfseRenderer) federal() {
	f := r.d.Valores.Trib.TribFed
	r.rule()
	r.title("TRIBUTAÇÃO FEDERAL")
	r.field(1, 1, "IRRF", brl(f.VRetIRRF))
	r.field(2, 1, "Contribuição Previdenciária - Retida", brl(f.VRetCP))
	r.field(3, 1, "CSLL - Retida", brl(f.VRetCSLL))
	r.next()
	r.field(0, 1, "PIS - Débito Apuração Própria", brl(f.Piscofins.VPis))
	r.field(1, 1, "COFINS - Débito Apuração Própria", brl(f.Piscofins.VCofins))
	r.field(2, 2, "Descrição Contrib. Sociais - Retidas", nfseRetPCC[f.Piscofins.TpRetPisCofins])
	r.next()
}

func (r *nfseRenderer) ibscbs() {
	r.rule()
	r.title("TRIBUTAÇÃO IBS / CBS")
	cst, class, op := "", "", ""
	if g := r.d.IBSCBS; g != nil {
		cst, class, op = g.Valores.Trib.CST, g.Valores.Trib.CClassTrib, g.CIndOp
		if t := g.Valores.Trib.G; t != nil && cst == "" {
			cst, class = t.CST, t.CClassTrib
		}
	}
	r.field(1, 1, "CST / CCLASSTRIB", joinNonEmpty(" / ", cst, class))
	g := r.n.IBSCBS
	if g == nil {
		g = &sIBSNF{}
	}
	incid := g.XLocalidadeIncid
	if m, ok := ibge.Lookup(g.CLocalidadeIncid); ok {
		incid = m.Name + " / " + m.UF
	}
	r.field(2, 2, "Indicador de Operação / Código IBGE / Município da Incidência", joinNonEmpty(" / ", op, g.CLocalidadeIncid, incid))
	r.next()
	v, t := g.Valores, g.TotCIBS
	r.field(0, 1, "Base de Cálculo do IBS/CBS", brl(v.VBC))
	r.field(1, 1, "Red. Alíquota IBS UF / Mun / CBS", joinNonEmpty(" / ", pct(v.UF.PRedAliqUF), pct(v.Mun.PRedAliqMun), pct(v.Fed.PRedAliqCBS)))
	r.field(2, 1, "Alíquota IBS UF / IBS Mun", joinNonEmpty(" / ", pct(v.UF.PIBSUF), pct(v.Mun.PIBSMun)))
	r.field(3, 1, "Alíquota CBS", pct(v.Fed.PCBS))
	r.next()
	r.field(0, 1, "Alíq. Efetiva Estadual - IBS", pct(v.UF.PAliqEfetUF))
	r.field(1, 1, "Valor Apurado Estadual - IBS", brl(t.GIBS.GIBSUFTot.VIBSUF))
	r.field(2, 1, "Alíq. Efetiva Municipal - IBS", pct(v.Mun.PAliqEfetMun))
	r.field(3, 1, "Valor Apurado Municipal - IBS", brl(t.GIBS.GIBSMunTot.VIBSMun))
	r.next()
	r.field(0, 1, "Valor Total Apurado - IBS", brl(t.GIBS.VIBSTot))
	r.field(1, 1, "Alíquota Efetiva - CBS", pct(v.Fed.PAliqEfetCBS))
	r.field(2, 1, "Valor Total Apurado - CBS", brl(t.GCBS.VCBS))
	r.next()
}

func (r *nfseRenderer) totals() {
	r.rule()
	r.title("VALOR TOTAL DA NFS-e")
	dv := r.d.Valores
	r.field(1, 1, "Valor da Operação / Serviço", brl(dv.VServPrest.VServ))
	r.field(2, 1, "Desconto Incondicionado", brl(dv.VDescCondIncond.VDescIncond))
	r.field(3, 1, "Desconto Condicionado", brl(dv.VDescCondIncond.VDescCond))
	r.next()
	var ibs, cbs, total string
	if g := r.n.IBSCBS; g != nil {
		ibs, cbs, total = g.TotCIBS.GIBS.VIBSTot, g.TotCIBS.GCBS.VCBS, g.TotCIBS.VTotNF
	}
	r.field(0, 1, "Total das Retenções (ISSQN / Federais)", brl(r.n.Valores.VTotalRet))
	r.field(1, 1, "Valor Líquido da NFS-e", brl(r.n.Valores.VLiq))
	r.field(2, 1, "Total do IBS / CBS", brl(sum(ibs, cbs)))
	r.page.FillRect(margin+3*sColW+0.2, r.y+0.2, sColW-0.4, sRow-0.4, 0.95)
	r.field(3, 1, "Valor Líquido da NFS-e + IBS/CBS", brl(total))
	r.next()
}

func (r *nfseRenderer) additional() {
	r.rule()
	r.page.Text(margin+0.8, r.y+0.8, "INFORMAÇÕES COMPLEMENTARES", sTitle)
	var parts []string
	if s := r.d.Subst; s != nil && s.ChSubstda != "" {
		parts = append(parts, "NFS-e substituída: "+s.ChSubstda)
	}
	if x := r.d.Serv.InfoCompl.XInfComp; x != "" {
		parts = append(parts, x)
	}
	if r.n.XOutInf != "" {
		parts = append(parts, r.n.XOutInf)
	}
	tt := r.d.Valores.Trib.TotTrib
	if tt.PTotTribSN != "" {
		parts = append(parts, "Tributos aproximados (Simples Nacional, Lei 12.741/2012): "+pct(tt.PTotTribSN))
	} else if v := tt.VTotTrib; v.VTotTribFed+v.VTotTribEst+v.VTotTribMun != "" {
		parts = append(parts, fmt.Sprintf("Totais aproximados dos tributos (Lei 12.741/2012): federais %s; estaduais %s; municipais %s",
			brl(v.VTotTribFed), brl(v.VTotTribEst), brl(v.VTotTribMun)))
	}
	r.page.TextWrapped(margin+0.8, r.y+4, width-1.6, dash(strings.Join(parts, " | ")), sValue, 10)
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" && p != "-" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// brl formata um valor monetário do XML ("1500.00") como "R$ 1.500,00"; vazio fica vazio.
func brl(s string) string {
	if s == "" {
		return ""
	}
	return "R$ " + money(s)
}

func pct(s string) string {
	if s == "" {
		return ""
	}
	return strings.Replace(s, ".", ",", 1) + "%"
}

func sum(a, b string) string {
	if a == "" && b == "" {
		return ""
	}
	x, _ := decimal.ParseAuto(or(a, "0"))
	y, _ := decimal.ParseAuto(or(b, "0"))
	total, err := x.Add(y)
	if err != nil {
		return ""
	}
	return total.String()
}

// cepNT é a máscara de CEP da NT 008/2026 (nn.nnn-nnn).
func cepNT(c string) string {
	if len(c) == 8 {
		return c[:2] + "." + c[2:5] + "-" + c[5:]
	}
	return c
}

func fone(f string) string {
	switch len(f) {
	case 10:
		return "(" + f[:2] + ") " + f[2:6] + "-" + f[6:]
	case 11:
		return "(" + f[:2] + ") " + f[2:7] + "-" + f[7:]
	}
	return f
}

func isoDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("02/01/2006")
}

func dateTimeMin(s string) string {
	t, err := time.Parse("2006-01-02T15:04:05-07:00", s)
	if err != nil {
		return s
	}
	return t.Format("02/01/2006 15:04")
}
