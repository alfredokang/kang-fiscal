package nfe

import "github.com/alfredokang/kang-fiscal/pkg/decimal"

// ComputeTotals soma os itens já calculados e devolve o grupo total.
//
// O total é a soma dos itens arredondados, nunca o arredondamento da soma (Constituição,
// Art. 5.3): é assim que a SEFAZ confere, e a diferença de um centavo é rejeição.
//
// O vNF segue a fórmula do manual para os tributos atuais. IBS e CBS NÃO entram no vNF em
// 2026: neste ano eles são destacados para teste e compensados com PIS/COFINS. O vNFTot
// (com IBS, CBS e IS) só é preenchido quando includeNFTot é verdadeiro — ele é opcional, e
// a regra de quando passa a ser exigido ainda vai sair em nota técnica.
func ComputeTotals(items []Det, includeNFTot bool) (Total, error) {
	var t ICMSTot
	var ibs ibsAcc
	var vIS, vTotTrib Dec
	hasIS := false

	add := func(dst *Dec, v *Dec) error {
		if v == nil {
			return nil
		}
		sum, err := dst.Add(*v)
		if err != nil {
			return err
		}
		*dst = sum
		return nil
	}

	for i := range items {
		it := &items[i]
		p := it.Prod
		if p.IndTot == 1 {
			v := p.VProd
			if err := add(&t.VProd, &v); err != nil {
				return Total{}, err
			}
		}
		for _, pair := range [][2]*Dec{{&t.VFrete, p.VFrete}, {&t.VSeg, p.VSeg}, {&t.VDesc, p.VDesc}, {&t.VOutro, p.VOutro}} {
			if err := add(pair[0], pair[1]); err != nil {
				return Total{}, err
			}
		}
		im := it.Imposto
		if err := add(&vTotTrib, im.VTotTrib); err != nil {
			return Total{}, err
		}
		if err := sumICMS(&t, im.ICMS, add); err != nil {
			return Total{}, err
		}
		if im.IPI != nil && im.IPI.IPITrib != nil {
			v := im.IPI.IPITrib.VIPI
			if err := add(&t.VIPI, &v); err != nil {
				return Total{}, err
			}
		}
		if v := pisValue(im.PIS); v != nil {
			if err := add(&t.VPIS, v); err != nil {
				return Total{}, err
			}
		}
		if v := cofinsValue(im.COFINS); v != nil {
			if err := add(&t.VCOFINS, v); err != nil {
				return Total{}, err
			}
		}
		if d := im.ICMSUFDest; d != nil {
			for _, pair := range []struct {
				dst **Dec
				v   *Dec
			}{{&t.VFCPUFDest, d.VFCPUFDest}, {&t.VICMSUFDest, &d.VICMSUFDest}, {&t.VICMSUFRemet, &d.VICMSUFRemet}} {
				if *pair.dst == nil {
					*pair.dst = &Dec{}
				}
				if err := add(*pair.dst, pair.v); err != nil {
					return Total{}, err
				}
			}
		}
		if im.IS != nil {
			hasIS = true
			if err := add(&vIS, im.IS.VIS); err != nil {
				return Total{}, err
			}
		}
		if err := ibs.add(im.IBSCBS); err != nil {
			return Total{}, err
		}
	}

	// vNF = vProd − vDesc + vST + vFCPST + vFrete + vSeg + vOutro + vII + vIPI + vIPIDevol
	vNF := t.VProd
	var err error
	if vNF, err = vNF.Sub(t.VDesc); err != nil {
		return Total{}, err
	}
	for _, v := range []Dec{t.VST, t.VFCPST, t.VFrete, t.VSeg, t.VOutro, t.VII, t.VIPI, t.VIPIDevol} {
		if vNF, err = vNF.Add(v); err != nil {
			return Total{}, err
		}
	}
	t.VNF = vNF
	if !vTotTrib.IsZero() {
		t.VTotTrib = &vTotTrib
	}

	total := Total{ICMSTot: normalize(t)}
	if hasIS {
		total.ISTot = &ISTot{VIS: money(vIS)}
	}
	if ibs.present {
		total.IBSCBSTot = ibs.total()
	}
	if includeNFTot {
		nfTot := total.ICMSTot.VNF
		for _, v := range []Dec{vIS, ibs.vIBS, ibs.vCBS} {
			if nfTot, err = nfTot.Add(v); err != nil {
				return Total{}, err
			}
		}
		nfTot = money(nfTot)
		total.VNFTot = &nfTot
	}
	return total, nil
}

func sumICMS(t *ICMSTot, icms *ICMS, add func(*Dec, *Dec) error) error {
	if icms == nil {
		return nil
	}
	type pair struct{ dst, v *Dec }
	var pairs []pair
	switch {
	case icms.ICMS00 != nil:
		g := icms.ICMS00
		pairs = []pair{{&t.VBC, &g.VBC}, {&t.VICMS, &g.VICMS}, {&t.VFCP, g.VFCP}}
	case icms.ICMS20 != nil:
		g := icms.ICMS20
		pairs = []pair{{&t.VBC, &g.VBC}, {&t.VICMS, &g.VICMS}, {&t.VFCP, g.VFCP}, {&t.VICMSDeson, g.VICMSDeson}}
	case icms.ICMS40 != nil:
		pairs = []pair{{&t.VICMSDeson, icms.ICMS40.VICMSDeson}}
	case icms.ICMS90 != nil:
		pairs = []pair{{&t.VBC, icms.ICMS90.VBC}, {&t.VICMS, icms.ICMS90.VICMS}}
	case icms.ICMSSN900 != nil:
		pairs = []pair{{&t.VBC, icms.ICMSSN900.VBC}, {&t.VICMS, icms.ICMSSN900.VICMS}}
	}
	for _, p := range pairs {
		if err := add(p.dst, p.v); err != nil {
			return err
		}
	}
	return nil
}

func pisValue(p *PIS) *Dec {
	switch {
	case p == nil:
		return nil
	case p.PISAliq != nil:
		return &p.PISAliq.VPIS
	case p.PISOutr != nil:
		return &p.PISOutr.VPIS
	}
	return nil
}

func cofinsValue(c *COFINS) *Dec {
	switch {
	case c == nil:
		return nil
	case c.COFINSAliq != nil:
		return &c.COFINSAliq.VCOFINS
	case c.COFINSOutr != nil:
		return &c.COFINSOutr.VCOFINS
	}
	return nil
}

// money leva a soma às 2 casas do leiaute. Soma de valores de 2 casas já tem 2 casas; isto
// só fixa a escala do zero ("0.00") para o XML ficar uniforme.
func money(v Dec) Dec {
	out, _ := v.Rescale(2, decimal.HalfUp)
	return out
}

func normalize(t ICMSTot) ICMSTot {
	for _, f := range []*Dec{&t.VBC, &t.VICMS, &t.VICMSDeson, &t.VFCP, &t.VBCST, &t.VST, &t.VFCPST,
		&t.VFCPSTRet, &t.VProd, &t.VFrete, &t.VSeg, &t.VDesc, &t.VII, &t.VIPI, &t.VIPIDevol,
		&t.VPIS, &t.VCOFINS, &t.VOutro, &t.VNF} {
		*f = money(*f)
	}
	for _, f := range []*Dec{t.VFCPUFDest, t.VICMSUFDest, t.VICMSUFRemet, t.VTotTrib} {
		if f != nil {
			*f = money(*f)
		}
	}
	return t
}

type ibsAcc struct {
	present                                      bool
	vBC, vIBSUF, vIBSMun, vIBS, vCBS             Dec
	difUF, difMun, difCBS, devUF, devMun, devCBS Dec
}

func (a *ibsAcc) add(g *IBSCBS) error {
	if g == nil {
		return nil
	}
	a.present = true
	if g.GIBSCBS == nil {
		return nil
	}
	c := g.GIBSCBS
	pairs := []struct{ dst, v *Dec }{
		{&a.vBC, &c.VBC}, {&a.vIBSUF, &c.GIBSUF.VIBSUF}, {&a.vIBSMun, &c.GIBSMun.VIBSMun},
		{&a.vIBS, &c.VIBS}, {&a.vCBS, &c.GCBS.VCBS},
	}
	if d := c.GIBSUF.GDif; d != nil {
		pairs = append(pairs, struct{ dst, v *Dec }{&a.difUF, &d.VDif})
	}
	if d := c.GIBSMun.GDif; d != nil {
		pairs = append(pairs, struct{ dst, v *Dec }{&a.difMun, &d.VDif})
	}
	if d := c.GCBS.GDif; d != nil {
		pairs = append(pairs, struct{ dst, v *Dec }{&a.difCBS, &d.VDif})
	}
	if d := c.GIBSUF.GDevTrib; d != nil {
		pairs = append(pairs, struct{ dst, v *Dec }{&a.devUF, &d.VDevTrib})
	}
	if d := c.GIBSMun.GDevTrib; d != nil {
		pairs = append(pairs, struct{ dst, v *Dec }{&a.devMun, &d.VDevTrib})
	}
	if d := c.GCBS.GDevTrib; d != nil {
		pairs = append(pairs, struct{ dst, v *Dec }{&a.devCBS, &d.VDevTrib})
	}
	for _, p := range pairs {
		sum, err := p.dst.Add(*p.v)
		if err != nil {
			return err
		}
		*p.dst = sum
	}
	return nil
}

func (a *ibsAcc) total() *IBSCBSTot {
	zero := money(Dec{})
	return &IBSCBSTot{
		VBCIBSCBS: money(a.vBC),
		GIBS: &GIBSTot{
			GIBSUF:           GIBSUFTot{VDif: money(a.difUF), VDevTrib: money(a.devUF), VIBSUF: money(a.vIBSUF)},
			GIBSMun:          GIBSMunTot{VDif: money(a.difMun), VDevTrib: money(a.devMun), VIBSMun: money(a.vIBSMun)},
			VIBS:             money(a.vIBS),
			VCredPres:        zero,
			VCredPresCondSus: zero,
		},
		GCBS: &GCBSTot{VDif: money(a.difCBS), VDevTrib: money(a.devCBS), VCBS: money(a.vCBS),
			VCredPres: zero, VCredPresCondSus: zero},
	}
}
