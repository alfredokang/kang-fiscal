// Package tax calcula os tributos de um item da NF-e: os atuais (ICMS, IPI, PIS, COFINS)
// e os da reforma (IBS e CBS), lado a lado, como a transição de 2026 a 2032 exige.
//
// Não decide enquadramento: quem diz o CST, o CSOSN e as alíquotas de ICMS, PIS e COFINS é
// o Profile, definido pelo contador da empresa. O que este pacote garante é a conta:
// base, alíquota, redução e arredondamento, sempre em ponto fixo e arredondando UMA vez por
// valor (Constituição, Art. 5).
//
// As alíquotas de IBS e CBS não estão aqui nem no Profile: vêm de Rates, que o chamador lê
// da tabela por vigência. Elas ainda vão mudar (Constituição, Art. 12).
package tax

import (
	"errors"
	"fmt"

	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
)

// Dec é o número de ponto fixo.
type Dec = decimal.Fixed

var (
	ErrUnsupportedCST = errors.New("tax: CST/CSOSN não suportado por este cálculo")
	ErrInvalidItem    = errors.New("tax: item inválido")
	ErrRegime         = errors.New("tax: CST do regime normal em empresa do Simples, ou CSOSN no regime normal")
)

// ICMSRule é o ICMS do perfil.
type ICMSRule struct {
	// CST (regime normal: 00, 20, 40, 41, 50, 60, 90) ou CSOSN (Simples: 101, 102, 103,
	// 300, 400, 500, 900).
	Code string
	// Rate é o pICMS (00, 20, 90, 900).
	Rate Dec
	// BaseReduction é o pRedBC, em percentual (20).
	BaseReduction Dec
	// FCPRate é o pFCP (fundo de combate à pobreza), quando a UF cobra.
	FCPRate Dec
	// SimplesCreditRate é o pCredSN (101), a alíquota de crédito que o Simples permite.
	SimplesCreditRate Dec
}

// PISCOFINSRule é o PIS ou a COFINS do perfil.
type PISCOFINSRule struct {
	CST  string // 01, 02 por alíquota; 04 a 09 não tributado; 49 a 99 outras
	Rate Dec    // pPIS ou pCOFINS
}

// IPIRule é o IPI do perfil. nil no Profile = não informa IPI (comércio, em geral).
type IPIRule struct {
	CST       string // 00, 49, 50, 99 tributado; 01 a 05, 51 a 55 não tributado
	Rate      Dec
	LegalCode string // cEnq, o código de enquadramento legal; "999" quando não há
}

// IBSCBSRule é o IBS/CBS do perfil. nil = não informa (permitido ao Simples em 2026).
type IBSCBSRule struct {
	CST       string // 000 tributação integral, 200 alíquota reduzida, 410 imunidade...
	ClassTrib string // cClassTrib da tabela oficial
	// RateReduction é o pRedAliq: o percentual de redução da alíquota dos regimes
	// diferenciados da LC 214/2025 (60% para alimentos da cesta estendida, por exemplo).
	RateReduction Dec
	// NoTax: CST de imunidade, isenção ou suspensão — o grupo vai sem gIBSCBS.
	NoTax bool
}

// Profile é a regra fiscal de uma operação, como o contador define.
type Profile struct {
	CFOPInternal   string // venda dentro da UF (5102)
	CFOPInterstate string // venda para outra UF (6102)
	ICMS           ICMSRule
	PIS            PISCOFINSRule
	COFINS         PISCOFINSRule
	IPI            *IPIRule
	IBSCBS         *IBSCBSRule
	// ExcludeICMSFromPISBase tira o ICMS destacado da base do PIS e da COFINS (STF, Tema
	// 69). É decisão do contador, não deste pacote; por isso não há valor padrão escondido.
	ExcludeICMSFromPISBase bool
}

// Rates são as alíquotas de IBS e CBS vigentes na data da emissão, para a UF e o município
// do fato gerador.
type Rates struct {
	CBS    Dec
	IBSUF  Dec
	IBSMun Dec
	// Version identifica a linha da tabela de onde vieram, para o snapshot da nota.
	Version string
}

// Item é a operação de um item.
type Item struct {
	Origin    int // orig do ICMS: 0 nacional, 1 estrangeira importação direta...
	Quantity  Dec // qCom
	UnitPrice Dec // vUnCom
	Discount  Dec // vDesc do item
	Freight   Dec // vFrete rateado no item
	Insurance Dec
	Other     Dec
}

// Context é o que da operação muda o cálculo.
type Context struct {
	Simples    bool // CRT 1, 2 ou 4
	Interstate bool // idDest = 2
}

// Result é o item calculado.
type Result struct {
	CFOP    string
	VProd   Dec
	Imposto nfe.Imposto
}

// Compute calcula os tributos do item.
//
// Bases:
//   - vProd = qCom × vUnCom, arredondado a 2 casas uma única vez.
//   - ICMS: vProd + frete + seguro + outras − desconto (com pRedBC no CST 20).
//   - PIS/COFINS: a mesma base, menos o ICMS se o perfil mandar excluir.
//   - IBS/CBS: a mesma base menos ICMS, FCP, PIS e COFINS — na transição esses tributos não
//     integram a base (LC 214/2025, art. 12, e a regra de validação da NT 2025.002).
func Compute(it Item, p Profile, r Rates, ctx Context) (Result, error) {
	if it.Quantity.IsNegative() || it.Quantity.IsZero() || it.UnitPrice.IsNegative() || it.Discount.IsNegative() {
		return Result{}, ErrInvalidItem
	}
	vProd, err := it.Quantity.Mul(it.UnitPrice, 2, decimal.HalfUp)
	if err != nil {
		return Result{}, err
	}
	base := vProd
	for _, v := range []Dec{it.Freight, it.Insurance, it.Other} {
		if base, err = base.Add(v); err != nil {
			return Result{}, err
		}
	}
	if base, err = base.Sub(it.Discount); err != nil {
		return Result{}, err
	}
	base = money(base)
	if base.IsNegative() {
		return Result{}, fmt.Errorf("%w: desconto maior que o valor do item", ErrInvalidItem)
	}

	res := Result{CFOP: p.CFOPInternal, VProd: vProd}
	if ctx.Interstate {
		res.CFOP = p.CFOPInterstate
	}

	icms, vICMS, vFCP, err := computeICMS(it.Origin, p.ICMS, base, ctx.Simples)
	if err != nil {
		return Result{}, err
	}
	res.Imposto.ICMS = icms

	pisBase := base
	if p.ExcludeICMSFromPISBase {
		if pisBase, err = base.Sub(vICMS); err != nil {
			return Result{}, err
		}
	}
	pis, vPIS, err := computePIS(p.PIS, pisBase)
	if err != nil {
		return Result{}, err
	}
	cofins, vCOFINS, err := computeCOFINS(p.COFINS, pisBase)
	if err != nil {
		return Result{}, err
	}
	res.Imposto.PIS, res.Imposto.COFINS = pis, cofins

	if p.IPI != nil {
		if res.Imposto.IPI, err = computeIPI(*p.IPI, base); err != nil {
			return Result{}, err
		}
	}

	if p.IBSCBS != nil {
		ibsBase := base
		for _, v := range []Dec{vICMS, vFCP, vPIS, vCOFINS} {
			if ibsBase, err = ibsBase.Sub(v); err != nil {
				return Result{}, err
			}
		}
		if res.Imposto.IBSCBS, err = computeIBSCBS(*p.IBSCBS, r, money(ibsBase)); err != nil {
			return Result{}, err
		}
	}
	return res, nil
}

func money(v Dec) Dec {
	out, _ := v.Rescale(2, decimal.HalfUp)
	return out
}

func rate4(v Dec) Dec {
	out, _ := v.Rescale(4, decimal.HalfUp)
	return out
}

func pct(base, rate Dec) (Dec, error) { return base.Percent(rate, 2, decimal.HalfUp) }

func computeICMS(orig int, r ICMSRule, base Dec, simples bool) (icms *nfe.ICMS, vICMS, vFCP Dec, err error) {
	isCSOSN := len(r.Code) == 3
	if isCSOSN != simples {
		return nil, Dec{}, Dec{}, ErrRegime
	}
	switch r.Code {
	case "00":
		v, err := pct(base, r.Rate)
		if err != nil {
			return nil, Dec{}, Dec{}, err
		}
		g := &nfe.ICMS00{Orig: orig, CST: "00", ModBC: 3, VBC: base, PICMS: rate4(r.Rate), VICMS: v}
		if !r.FCPRate.IsZero() {
			f, err := pct(base, r.FCPRate)
			if err != nil {
				return nil, Dec{}, Dec{}, err
			}
			g.PFCP, g.VFCP, vFCP = rate4(r.FCPRate).Ptr(), f.Ptr(), f
		}
		return &nfe.ICMS{ICMS00: g}, v, vFCP, nil
	case "20":
		reduced, err := base.Percent(subFrom100(r.BaseReduction), 2, decimal.HalfUp)
		if err != nil {
			return nil, Dec{}, Dec{}, err
		}
		v, err := pct(reduced, r.Rate)
		if err != nil {
			return nil, Dec{}, Dec{}, err
		}
		return &nfe.ICMS{ICMS20: &nfe.ICMS20{Orig: orig, CST: "20", ModBC: 3, PRedBC: rate4(r.BaseReduction),
			VBC: reduced, PICMS: rate4(r.Rate), VICMS: v}}, v, Dec{}, nil
	case "40", "41", "50":
		return &nfe.ICMS{ICMS40: &nfe.ICMS40{Orig: orig, CST: r.Code}}, Dec{}, Dec{}, nil
	case "60":
		return &nfe.ICMS{ICMS60: &nfe.ICMS60{Orig: orig, CST: "60"}}, Dec{}, Dec{}, nil
	case "90":
		if r.Rate.IsZero() {
			return &nfe.ICMS{ICMS90: &nfe.ICMS90{Orig: orig, CST: "90"}}, Dec{}, Dec{}, nil
		}
		v, err := pct(base, r.Rate)
		if err != nil {
			return nil, Dec{}, Dec{}, err
		}
		mod := 3
		return &nfe.ICMS{ICMS90: &nfe.ICMS90{Orig: orig, CST: "90", ModBC: &mod, VBC: base.Ptr(),
			PICMS: rate4(r.Rate).Ptr(), VICMS: v.Ptr()}}, v, Dec{}, nil
	case "101":
		credit, err := pct(base, r.SimplesCreditRate)
		if err != nil {
			return nil, Dec{}, Dec{}, err
		}
		return &nfe.ICMS{ICMSSN101: &nfe.ICMSSN101{Orig: orig, CSOSN: "101",
			PCredSN: rate4(r.SimplesCreditRate), VCredICMSSN: credit}}, Dec{}, Dec{}, nil
	case "102", "103", "300", "400":
		return &nfe.ICMS{ICMSSN102: &nfe.ICMSSN102{Orig: orig, CSOSN: r.Code}}, Dec{}, Dec{}, nil
	case "500":
		return &nfe.ICMS{ICMSSN500: &nfe.ICMSSN500{Orig: orig, CSOSN: "500"}}, Dec{}, Dec{}, nil
	case "900":
		if r.Rate.IsZero() {
			return &nfe.ICMS{ICMSSN900: &nfe.ICMSSN900{Orig: orig, CSOSN: "900"}}, Dec{}, Dec{}, nil
		}
		v, err := pct(base, r.Rate)
		if err != nil {
			return nil, Dec{}, Dec{}, err
		}
		mod := 3
		return &nfe.ICMS{ICMSSN900: &nfe.ICMSSN900{Orig: orig, CSOSN: "900", ModBC: &mod, VBC: base.Ptr(),
			PICMS: rate4(r.Rate).Ptr(), VICMS: v.Ptr()}}, v, Dec{}, nil
	}
	return nil, Dec{}, Dec{}, fmt.Errorf("%w: ICMS %q", ErrUnsupportedCST, r.Code)
}

// subFrom100 devolve 100 − p, para aplicar uma redução percentual.
func subFrom100(p Dec) Dec {
	out, _ := decimal.MustNew(100, 0).Sub(p)
	return out
}

func computePIS(r PISCOFINSRule, base Dec) (*nfe.PIS, Dec, error) {
	switch {
	case r.CST == "01" || r.CST == "02":
		v, err := pct(base, r.Rate)
		return &nfe.PIS{PISAliq: &nfe.PISAliq{CST: r.CST, VBC: base, PPIS: rate4(r.Rate), VPIS: v}}, v, err
	case r.CST >= "04" && r.CST <= "09":
		return &nfe.PIS{PISNT: &nfe.PISNT{CST: r.CST}}, Dec{}, nil
	case r.CST >= "49" && r.CST <= "99":
		b, v := money(Dec{}), money(Dec{})
		var err error
		if !r.Rate.IsZero() {
			b = base
			v, err = pct(base, r.Rate)
		}
		return &nfe.PIS{PISOutr: &nfe.PISOutr{CST: r.CST, VBC: b, PPIS: rate4(r.Rate), VPIS: v}}, v, err
	}
	return nil, Dec{}, fmt.Errorf("%w: PIS %q", ErrUnsupportedCST, r.CST)
}

func computeCOFINS(r PISCOFINSRule, base Dec) (*nfe.COFINS, Dec, error) {
	pis, v, err := computePIS(r, base)
	if err != nil {
		return nil, Dec{}, fmt.Errorf("%w (COFINS)", err)
	}
	switch {
	case pis.PISAliq != nil:
		g := pis.PISAliq
		return &nfe.COFINS{COFINSAliq: &nfe.COFINSAliq{CST: g.CST, VBC: g.VBC, PCOFINS: g.PPIS, VCOFINS: g.VPIS}}, v, nil
	case pis.PISNT != nil:
		return &nfe.COFINS{COFINSNT: &nfe.COFINSNT{CST: pis.PISNT.CST}}, v, nil
	default:
		g := pis.PISOutr
		return &nfe.COFINS{COFINSOutr: &nfe.COFINSOutr{CST: g.CST, VBC: g.VBC, PCOFINS: g.PPIS, VCOFINS: g.VPIS}}, v, nil
	}
}

func computeIPI(r IPIRule, base Dec) (*nfe.IPI, error) {
	enq := r.LegalCode
	if enq == "" {
		enq = "999"
	}
	switch r.CST {
	case "00", "49", "50", "99":
		v, err := pct(base, r.Rate)
		return &nfe.IPI{CEnq: enq, IPITrib: &nfe.IPITrib{CST: r.CST, VBC: base, PIPI: rate4(r.Rate), VIPI: v}}, err
	case "01", "02", "03", "04", "05", "51", "52", "53", "54", "55":
		return &nfe.IPI{CEnq: enq, IPINT: &nfe.IPINT{CST: r.CST}}, nil
	}
	return nil, fmt.Errorf("%w: IPI %q", ErrUnsupportedCST, r.CST)
}

// computeIBSCBS aplica as alíquotas vigentes, com a redução do regime diferenciado quando o
// perfil tem. A alíquota efetiva é p × (1 − pRedAliq/100), arredondada a 4 casas como o
// leiaute exige, e o valor sai dela.
func computeIBSCBS(r IBSCBSRule, rates Rates, base Dec) (*nfe.IBSCBS, error) {
	g := &nfe.IBSCBS{CST: r.CST, CClassTrib: r.ClassTrib}
	if r.NoTax {
		return g, nil
	}
	part := func(rate Dec) (Dec, *nfe.GRed, Dec, error) {
		effective := rate4(rate)
		var red *nfe.GRed
		if !r.RateReduction.IsZero() {
			e, err := rate.Percent(subFrom100(r.RateReduction), 4, decimal.HalfUp)
			if err != nil {
				return Dec{}, nil, Dec{}, err
			}
			effective = e
			red = &nfe.GRed{PRedAliq: rate4(r.RateReduction), PAliqEfet: e}
		}
		v, err := pct(base, effective)
		return rate4(rate), red, v, err
	}
	pUF, redUF, vUF, err := part(rates.IBSUF)
	if err != nil {
		return nil, err
	}
	pMun, redMun, vMun, err := part(rates.IBSMun)
	if err != nil {
		return nil, err
	}
	pCBS, redCBS, vCBS, err := part(rates.CBS)
	if err != nil {
		return nil, err
	}
	vIBS, err := vUF.Add(vMun)
	if err != nil {
		return nil, err
	}
	g.GIBSCBS = &nfe.GIBSCBS{
		VBC:     base,
		GIBSUF:  nfe.GIBSUF{PIBSUF: pUF, GRed: redUF, VIBSUF: vUF},
		GIBSMun: nfe.GIBSMun{PIBSMun: pMun, GRed: redMun, VIBSMun: vMun},
		VIBS:    money(vIBS),
		GCBS:    nfe.GCBS{PCBS: pCBS, GRed: redCBS, VCBS: vCBS},
	}
	return g, nil
}
