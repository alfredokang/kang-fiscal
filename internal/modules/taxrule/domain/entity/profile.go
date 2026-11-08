// Package entity contém o perfil tributário e a alíquota por vigência.
package entity

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
)

var codePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)

// Profile é uma regra fiscal da empresa: "venda de café dentro do estado", "venda de livro".
//
// O produto aponta para um perfil pelo código, e quem define o perfil é o contador. Calcular
// tributo só pelo NCM, para o país inteiro, é impossível de acertar: o mesmo NCM tem CST e
// alíquota diferentes conforme o enquadramento de quem vende.
type Profile struct {
	TenantID  string
	Code      string
	Name      string
	Rules     tax.Profile
	UpdatedAt time.Time
}

// NewProfile valida o perfil.
//
// A validação faz um cálculo de ensaio com o próprio motor: um CST que o motor não sabe
// calcular é recusado aqui, no cadastro, e não na primeira nota.
func NewProfile(tenantID, code, name string, rules tax.Profile, now time.Time) (*Profile, error) {
	code = strings.TrimSpace(code)
	if !codePattern.MatchString(code) {
		return nil, domain.ErrInvalidCode
	}
	name = strings.Join(strings.Fields(name), " ")
	if tenantID == "" || name == "" || rules.CFOPInternal == "" || rules.CFOPInterstate == "" {
		return nil, domain.ErrRequiredField
	}
	for _, cfop := range []string{rules.CFOPInternal, rules.CFOPInterstate} {
		if len(cfop) != 4 || strings.Trim(cfop, "0123456789") != "" {
			return nil, fmt.Errorf("%w: CFOP %q", domain.ErrInvalidProfile, cfop)
		}
	}
	if err := dryRun(rules); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidProfile, err)
	}
	return &Profile{TenantID: tenantID, Code: code, Name: name, Rules: rules, UpdatedAt: now}, nil
}

// IsSimples diz se o perfil é de empresa do Simples (ICMS por CSOSN).
func (p *Profile) IsSimples() bool { return len(p.Rules.ICMS.Code) == 3 }

func dryRun(rules tax.Profile) error {
	one := decimal.MustNew(1, 0)
	zero := decimal.MustNew(0, 4)
	_, err := tax.Compute(tax.Item{Quantity: one, UnitPrice: decimal.MustNew(100, 0)}, rules,
		tax.Rates{CBS: zero, IBSUF: zero, IBSMun: zero}, tax.Context{Simples: len(rules.ICMS.Code) == 3})
	return err
}
