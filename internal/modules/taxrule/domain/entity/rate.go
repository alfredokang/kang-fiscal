package entity

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
)

// RateKind é o tributo da alíquota.
type RateKind string

const (
	CBS    RateKind = "cbs"
	IBSUF  RateKind = "ibs_uf"
	IBSMun RateKind = "ibs_mun"
)

// Rate é uma alíquota da reforma com vigência.
//
// Local vazio é a alíquota geral: em 2026 o IBS de teste é o mesmo em todas as UFs. Quando
// cada UF e cada município fixar a sua, entram linhas com o local preenchido, e elas
// prevalecem sobre a geral.
type Rate struct {
	ID        string
	Kind      RateKind
	UF        string // só em ibs_uf; vazio = geral
	CityCode  string // só em ibs_mun; vazio = geral
	Value     decimal.Fixed
	ValidFrom time.Time // primeiro dia
	ValidTo   *time.Time
	Source    string // a norma que fixou a alíquota
}

// NewRate valida a alíquota.
func NewRate(kind RateKind, uf, city string, value decimal.Fixed, from time.Time, to *time.Time, source string) (*Rate, error) {
	switch kind {
	case CBS:
		uf, city = "", ""
	case IBSUF:
		city = ""
	case IBSMun:
		uf = ""
	default:
		return nil, domain.ErrRequiredField
	}
	if value.IsNegative() || value.Cmp(decimal.MustNew(100, 0)) > 0 || value.Scale() > 4 {
		return nil, domain.ErrInvalidRate
	}
	if to != nil && to.Before(from) {
		return nil, domain.ErrInvalidPeriod
	}
	if strings.TrimSpace(source) == "" {
		return nil, domain.ErrRequiredField
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	v, _ := value.Rescale(4, decimal.HalfUp)
	return &Rate{ID: id.String(), Kind: kind, UF: strings.ToUpper(uf), CityCode: city, Value: v,
		ValidFrom: day(from), ValidTo: dayPtr(to), Source: strings.TrimSpace(source)}, nil
}

// Covers diz se a alíquota vale na data.
func (r *Rate) Covers(date time.Time) bool {
	d := day(date)
	return !d.Before(r.ValidFrom) && (r.ValidTo == nil || !d.After(*r.ValidTo))
}

// Resolve escolhe, entre as alíquotas candidatas, a de cada tributo para a data e o local:
// a mais específica vence a geral. Faltando qualquer uma das três, a resposta é ErrNoRate.
func Resolve(candidates []*Rate, date time.Time, uf, city string) (tax.Rates, error) {
	pick := func(kind RateKind, local func(*Rate) string, want string) *Rate {
		var general, specific *Rate
		for _, r := range candidates {
			if r.Kind != kind || !r.Covers(date) {
				continue
			}
			if l := local(r); want != "" && l == want {
				specific = r
			} else if l == "" {
				general = r
			}
		}
		if specific != nil {
			return specific
		}
		return general
	}
	cbs := pick(CBS, func(*Rate) string { return "" }, "")
	ibsUF := pick(IBSUF, func(r *Rate) string { return r.UF }, strings.ToUpper(uf))
	ibsMun := pick(IBSMun, func(r *Rate) string { return r.CityCode }, city)
	if cbs == nil || ibsUF == nil || ibsMun == nil {
		return tax.Rates{}, domain.ErrNoRate
	}
	return tax.Rates{CBS: cbs.Value, IBSUF: ibsUF.Value, IBSMun: ibsMun.Value,
		Version: cbs.ID + "," + ibsUF.ID + "," + ibsMun.ID}, nil
}

func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func dayPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := day(*t)
	return &d
}
