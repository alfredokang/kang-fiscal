package handler

import (
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/adapter/http/request"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/adapter/http/response"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
)

func toRules(r request.Rules) tax.Profile {
	p := tax.Profile{
		CFOPInternal: r.CFOPInternal, CFOPInterstate: r.CFOPInterstate, ExcludeICMSFromPISBase: r.ExcludeICMSFromPISBase,
		ICMS: tax.ICMSRule{Code: r.ICMS.Code, Rate: r.ICMS.Rate, BaseReduction: r.ICMS.BaseReduction,
			FCPRate: r.ICMS.FCPRate, SimplesCreditRate: r.ICMS.SimplesCreditRate},
		PIS:    tax.PISCOFINSRule{CST: r.PIS.CST, Rate: r.PIS.Rate},
		COFINS: tax.PISCOFINSRule{CST: r.COFINS.CST, Rate: r.COFINS.Rate},
	}
	if r.IPI != nil {
		p.IPI = &tax.IPIRule{CST: r.IPI.CST, Rate: r.IPI.Rate, LegalCode: r.IPI.LegalCode}
	}
	if r.IBSCBS != nil {
		p.IBSCBS = &tax.IBSCBSRule{CST: r.IBSCBS.CST, ClassTrib: r.IBSCBS.ClassTrib,
			RateReduction: r.IBSCBS.RateReduction, NoTax: r.IBSCBS.NoTax}
	}
	return p
}

func fromRules(p tax.Profile) request.Rules {
	r := request.Rules{
		CFOPInternal: p.CFOPInternal, CFOPInterstate: p.CFOPInterstate, ExcludeICMSFromPISBase: p.ExcludeICMSFromPISBase,
		ICMS: request.ICMS{Code: p.ICMS.Code, Rate: p.ICMS.Rate, BaseReduction: p.ICMS.BaseReduction,
			FCPRate: p.ICMS.FCPRate, SimplesCreditRate: p.ICMS.SimplesCreditRate},
		PIS:    request.PIS{CST: p.PIS.CST, Rate: p.PIS.Rate},
		COFINS: request.PIS{CST: p.COFINS.CST, Rate: p.COFINS.Rate},
	}
	if p.IPI != nil {
		r.IPI = &request.IPI{CST: p.IPI.CST, Rate: p.IPI.Rate, LegalCode: p.IPI.LegalCode}
	}
	if p.IBSCBS != nil {
		r.IBSCBS = &request.IBSCBS{CST: p.IBSCBS.CST, ClassTrib: p.IBSCBS.ClassTrib,
			RateReduction: p.IBSCBS.RateReduction, NoTax: p.IBSCBS.NoTax}
	}
	return r
}

func toProfileResponse(p *entity.Profile) response.Profile {
	return response.Profile{Code: p.Code, Name: p.Name, Rules: fromRules(p.Rules), UpdatedAt: p.UpdatedAt}
}

func toRateResponse(r *entity.Rate) response.Rate {
	out := response.Rate{ID: r.ID, Kind: string(r.Kind), UF: r.UF, CityCode: r.CityCode, Value: r.Value,
		ValidFrom: r.ValidFrom.Format("2006-01-02"), Source: r.Source}
	if r.ValidTo != nil {
		s := r.ValidTo.Format("2006-01-02")
		out.ValidTo = &s
	}
	return out
}

func ptr(v decimal.Fixed) *decimal.Fixed { return &v }

// flattenICMS transforma o grupo do CST num formato único, para a API não obrigar quem
// consome a conhecer os nove grupos do leiaute.
func flattenICMS(i *nfe.ICMS) *response.Tax {
	switch {
	case i == nil:
		return nil
	case i.ICMS00 != nil:
		return &response.Tax{Code: i.ICMS00.CST, Base: ptr(i.ICMS00.VBC), Rate: ptr(i.ICMS00.PICMS), Value: ptr(i.ICMS00.VICMS)}
	case i.ICMS20 != nil:
		return &response.Tax{Code: i.ICMS20.CST, Base: ptr(i.ICMS20.VBC), Rate: ptr(i.ICMS20.PICMS), Value: ptr(i.ICMS20.VICMS)}
	case i.ICMS40 != nil:
		return &response.Tax{Code: i.ICMS40.CST}
	case i.ICMS60 != nil:
		return &response.Tax{Code: i.ICMS60.CST}
	case i.ICMS90 != nil:
		return &response.Tax{Code: i.ICMS90.CST, Base: i.ICMS90.VBC, Rate: i.ICMS90.PICMS, Value: i.ICMS90.VICMS}
	case i.ICMSSN101 != nil:
		return &response.Tax{Code: i.ICMSSN101.CSOSN, Rate: ptr(i.ICMSSN101.PCredSN), Value: ptr(i.ICMSSN101.VCredICMSSN)}
	case i.ICMSSN102 != nil:
		return &response.Tax{Code: i.ICMSSN102.CSOSN}
	case i.ICMSSN500 != nil:
		return &response.Tax{Code: i.ICMSSN500.CSOSN}
	case i.ICMSSN900 != nil:
		return &response.Tax{Code: i.ICMSSN900.CSOSN, Base: i.ICMSSN900.VBC, Rate: i.ICMSSN900.PICMS, Value: i.ICMSSN900.VICMS}
	}
	return nil
}

func flattenPIS(p *nfe.PIS) *response.Tax {
	switch {
	case p == nil:
		return nil
	case p.PISAliq != nil:
		g := p.PISAliq
		return &response.Tax{Code: g.CST, Base: ptr(g.VBC), Rate: ptr(g.PPIS), Value: ptr(g.VPIS)}
	case p.PISOutr != nil:
		g := p.PISOutr
		return &response.Tax{Code: g.CST, Base: ptr(g.VBC), Rate: ptr(g.PPIS), Value: ptr(g.VPIS)}
	case p.PISNT != nil:
		return &response.Tax{Code: p.PISNT.CST}
	}
	return nil
}

func flattenCOFINS(c *nfe.COFINS) *response.Tax {
	switch {
	case c == nil:
		return nil
	case c.COFINSAliq != nil:
		g := c.COFINSAliq
		return &response.Tax{Code: g.CST, Base: ptr(g.VBC), Rate: ptr(g.PCOFINS), Value: ptr(g.VCOFINS)}
	case c.COFINSOutr != nil:
		g := c.COFINSOutr
		return &response.Tax{Code: g.CST, Base: ptr(g.VBC), Rate: ptr(g.PCOFINS), Value: ptr(g.VCOFINS)}
	case c.COFINSNT != nil:
		return &response.Tax{Code: c.COFINSNT.CST}
	}
	return nil
}

func flattenIPI(i *nfe.IPI) *response.Tax {
	switch {
	case i == nil:
		return nil
	case i.IPITrib != nil:
		g := i.IPITrib
		return &response.Tax{Code: g.CST, Base: ptr(g.VBC), Rate: ptr(g.PIPI), Value: ptr(g.VIPI)}
	case i.IPINT != nil:
		return &response.Tax{Code: i.IPINT.CST}
	}
	return nil
}

func flattenIBSCBS(g *nfe.IBSCBS) *response.IBSCBS {
	if g == nil {
		return nil
	}
	out := &response.IBSCBS{CST: g.CST, ClassTrib: g.CClassTrib}
	if c := g.GIBSCBS; c != nil {
		eff := func(p decimal.Fixed, red *nfe.GRed) *decimal.Fixed {
			if red != nil {
				return ptr(red.PAliqEfet)
			}
			return ptr(p)
		}
		out.Base = ptr(c.VBC)
		out.IBSUF = &response.Tax{Rate: eff(c.GIBSUF.PIBSUF, c.GIBSUF.GRed), Value: ptr(c.GIBSUF.VIBSUF)}
		out.IBSMun = &response.Tax{Rate: eff(c.GIBSMun.PIBSMun, c.GIBSMun.GRed), Value: ptr(c.GIBSMun.VIBSMun)}
		out.CBS = &response.Tax{Rate: eff(c.GCBS.PCBS, c.GCBS.GRed), Value: ptr(c.GCBS.VCBS)}
		out.IBS = ptr(c.VIBS)
	}
	return out
}

func toSimulationResponse(r usecase.SimulationResult) response.Simulation {
	out := response.Simulation{Rates: response.Rates{CBS: r.Rates.CBS, IBSUF: r.Rates.IBSUF, IBSMun: r.Rates.IBSMun}}
	for _, it := range r.Items {
		im := it.Imposto
		out.Items = append(out.Items, response.Item{CFOP: it.CFOP, VProd: it.VProd, ICMS: flattenICMS(im.ICMS),
			PIS: flattenPIS(im.PIS), COFINS: flattenCOFINS(im.COFINS), IPI: flattenIPI(im.IPI), IBSCBS: flattenIBSCBS(im.IBSCBS)})
	}
	t := r.Total.ICMSTot
	out.Totals = response.Totals{VProd: t.VProd, VDesc: t.VDesc, VICMS: t.VICMS, VIPI: t.VIPI, VPIS: t.VPIS,
		VCOFINS: t.VCOFINS, VNF: t.VNF, VNFTot: r.Total.VNFTot}
	if g := r.Total.IBSCBSTot; g != nil {
		out.Totals.VIBS, out.Totals.VCBS = ptr(g.GIBS.VIBS), ptr(g.GCBS.VCBS)
	}
	return out
}
