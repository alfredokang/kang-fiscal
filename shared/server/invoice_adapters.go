package server

import (
	"context"
	"errors"
	"time"

	certcmd "github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/command"
	certusecase "github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/usecase"
	certdomain "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	invoiceout "github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/port/output"
	invoicedomain "github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/service"
	issuerquery "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/query"
	issuerusecase "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/usecase"
	issuerdomain "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
	numberingusecase "github.com/alfredokang/kang-fiscal/internal/modules/numbering/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain/entity"
	taxusecase "github.com/alfredokang/kang-fiscal/internal/modules/taxrule/application/usecase"
	taxdomain "github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
)

// Os adapters abaixo implementam as portas da emissão sobre os outros módulos. Cada um
// traduz os erros do módulo dono para os da emissão: o handler da nota só conhece os
// próprios erros (Constituição, Art. 1.2).

type invoiceIssuerAdapter struct{ issuers *issuerusecase.IssuerService }

func (a invoiceIssuerAdapter) IssuerForInvoice(ctx context.Context, tenantID, issuerRef string) (service.IssuerSnapshot, []string, error) {
	d, err := a.issuers.Get(ctx, issuerquery.GetIssuer{TenantID: tenantID, IssuerID: issuerRef})
	if errors.Is(err, issuerdomain.ErrIssuerNotFound) || errors.Is(err, issuerdomain.ErrInvalidIssuerID) {
		return service.IssuerSnapshot{}, nil, invoicedomain.ErrIssuerNotFound
	}
	if err != nil {
		return service.IssuerSnapshot{}, nil, err
	}
	a2 := d.Address
	return service.IssuerSnapshot{CNPJ: d.CNPJ, Name: d.LegalName, TradeName: d.TradeName, IE: d.StateRegistration,
		CRT: d.TaxRegime, Environment: d.Environment,
		Address: service.Address{Street: a2.Street, Number: a2.Number, Complement: a2.Complement, District: a2.District,
			CityCode: a2.CityCode, CityName: a2.CityName, UF: a2.UF, ZipCode: a2.ZipCode, Phone: a2.Phone}}, d.Missing, nil
}

type invoiceSignerAdapter struct {
	certs *certusecase.CertificateService
}

func (a invoiceSignerAdapter) SignerFor(ctx context.Context, tenantID, issuerRef string) (*certificate.Certificate, error) {
	c, err := a.certs.Signer(ctx, certcmd.IssuerCertificate{TenantID: tenantID, IssuerRef: issuerRef})
	if errors.Is(err, certdomain.ErrNotFound) || errors.Is(err, certdomain.ErrExpired) || errors.Is(err, certdomain.ErrInvalidID) {
		return nil, invoicedomain.ErrNoCertificate
	}
	return c, err
}

type invoiceTaxAdapter struct{ taxes *taxusecase.TaxRuleService }

func (a invoiceTaxAdapter) TaxProfile(ctx context.Context, tenantID, code string) (tax.Profile, error) {
	p, err := a.taxes.GetProfile(ctx, tenantID, code)
	if errors.Is(err, taxdomain.ErrProfileNotFound) {
		return tax.Profile{}, invoicedomain.ErrTaxProfileNotFound
	}
	if err != nil {
		return tax.Profile{}, err
	}
	return p.Rules, nil
}

func (a invoiceTaxAdapter) TaxRatesAt(ctx context.Context, date time.Time, uf, city string) (tax.Rates, error) {
	r, err := a.taxes.ResolveRates(ctx, date, uf, city)
	if errors.Is(err, taxdomain.ErrNoRate) {
		return tax.Rates{}, invoicedomain.ErrTaxRateMissing
	}
	return r, err
}

type invoiceNumberAdapter struct {
	numbers *numberingusecase.NumberingService
}

func (a invoiceNumberAdapter) ReserveNumber(ctx context.Context, k invoiceout.NumberKey, documentRef string) (string, int, error) {
	r, err := a.numbers.Reserve(ctx, entity.SeriesKey{TenantID: k.TenantID, IssuerRef: k.IssuerRef, Model: k.Model,
		Series: k.Series, Environment: k.Environment}, documentRef)
	return r.ID, r.Number, err
}

func (a invoiceNumberAdapter) MarkNumberUsed(ctx context.Context, tenantID, reservationRef string) error {
	return a.numbers.MarkUsed(ctx, tenantID, reservationRef)
}
