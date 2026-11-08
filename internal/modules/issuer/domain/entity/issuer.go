// Package entity contém o agregado Issuer.
package entity

import (
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/valueobject"
	"github.com/alfredokang/kang-fiscal/pkg/document"
)

// Address é o endereço do CNPJ — que em muita empresa não é de onde a mercadoria sai.
type Address struct {
	Street     string
	Number     string
	Complement string
	District   string
	CityCode   vo.CityCode
	CityName   string
	UF         vo.UF
	ZipCode    string
	Phone      string
}

// FiscalData é o que o emitente pode corrigir depois do cadastro.
//
// O CNPJ fica de fora de propósito: trocar o CNPJ de um emitente é outro emitente. As notas
// já emitidas carregam o CNPJ antigo na chave de acesso, e um emitente que muda de CNPJ
// deixaria de achar as próprias notas.
type FiscalData struct {
	LegalName             string
	TradeName             string
	StateRegistration     vo.StateRegistration
	MunicipalRegistration string
	CNAE                  string
	TaxRegime             vo.TaxRegime
	Address               Address
}

// Issuer é a empresa em nome de quem a nota sai.
type Issuer struct {
	id          vo.IssuerID
	tenantID    string
	cnpj        document.CNPJ
	data        FiscalData
	environment vo.Environment
	createdAt   time.Time
	updatedAt   time.Time
}

// Snapshot é a leitura completa do agregado, para mapper e persistência.
type Snapshot struct {
	ID          vo.IssuerID
	TenantID    string
	CNPJ        document.CNPJ
	Data        FiscalData
	Environment vo.Environment
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Snapshot devolve uma cópia do estado.
func (i *Issuer) Snapshot() Snapshot {
	return Snapshot{
		ID: i.id, TenantID: i.tenantID, CNPJ: i.cnpj, Data: i.data,
		Environment: i.environment, CreatedAt: i.createdAt, UpdatedAt: i.updatedAt,
	}
}

// Restore reconstrói o agregado a partir do que foi persistido. Só a factory e o
// repositório chamam: não valida, porque o que está no banco já passou pela validação.
func Restore(s Snapshot) *Issuer {
	return &Issuer{
		id: s.ID, tenantID: s.TenantID, cnpj: s.CNPJ, data: s.Data,
		environment: s.Environment, createdAt: s.CreatedAt, updatedAt: s.UpdatedAt,
	}
}

// ID devolve o id.
func (i *Issuer) ID() vo.IssuerID { return i.id }

// TenantID devolve o tenant dono.
func (i *Issuer) TenantID() string { return i.tenantID }

// Environment devolve o ambiente em que o emitente emite.
func (i *Issuer) Environment() vo.Environment { return i.environment }

// UpdateFiscalData substitui os dados corrigíveis.
func (i *Issuer) UpdateFiscalData(data FiscalData, now time.Time) error {
	if err := validateRequired(data); err != nil {
		return err
	}
	i.data = data
	i.updatedAt = now
	return nil
}

// Missing lista o que falta para a SEFAZ aceitar uma nota deste emitente.
//
// O cadastro aceita entrar incompleto — a empresa cadastra hoje e descobre a inscrição
// estadual amanhã. O que trava é a EMISSÃO, e a lista diz exatamente o quê, em vez de
// deixar a pessoa descobrir pela rejeição da SEFAZ, uma de cada vez.
func (i *Issuer) Missing() []string {
	var missing []string
	add := func(ok bool, field string) {
		if !ok {
			missing = append(missing, field)
		}
	}
	a := i.data.Address
	add(i.data.StateRegistration != "", "state_registration")
	add(a.Street != "", "address.street")
	add(a.Number != "", "address.number")
	add(a.District != "", "address.district")
	add(a.CityCode != "", "address.city_code")
	add(a.CityName != "", "address.city_name")
	add(a.ZipCode != "", "address.zip_code")
	return missing
}

// CanIssue diz se o cadastro está completo para emitir.
func (i *Issuer) CanIssue() bool { return len(i.Missing()) == 0 }

// PromoteToProduction passa o emitente a emitir notas com validade jurídica.
//
// É um caso de uso próprio, e não um campo editável, porque o padrão é o erro barato
// (Constituição, Art. 6.4): nota de produção emitida por engano custa um cancelamento.
// A volta para homologação não existe — quem já emite em produção e quer testar usa outro
// emitente em homologação.
func (i *Issuer) PromoteToProduction(now time.Time) error {
	if i.environment == vo.Production {
		return domain.ErrAlreadyInProduction
	}
	if !i.CanIssue() {
		return domain.ErrIncompleteRegistration
	}
	i.environment = vo.Production
	i.updatedAt = now
	return nil
}

// validateRequired confere o mínimo sem o qual o emitente nem pode existir.
func validateRequired(d FiscalData) error {
	if d.LegalName == "" || d.Address.UF == "" {
		return domain.ErrRequiredField
	}
	if _, err := vo.ParseTaxRegime(int(d.TaxRegime)); err != nil {
		return err
	}
	return nil
}

// New é chamada pela factory.
func New(id vo.IssuerID, tenantID string, cnpj document.CNPJ, data FiscalData, now time.Time) (*Issuer, error) {
	if tenantID == "" {
		return nil, domain.ErrRequiredField
	}
	if err := validateRequired(data); err != nil {
		return nil, err
	}
	return &Issuer{
		id: id, tenantID: tenantID, cnpj: cnpj, data: data,
		environment: vo.Homologation, createdAt: now, updatedAt: now,
	}, nil
}
