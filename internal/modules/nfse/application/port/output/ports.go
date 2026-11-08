// Package output declara o que a NFS-e precisa dos outros módulos e do mundo de fora.
package output

import (
	"context"

	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfse"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// Issuer é o prestador, congelado na DPS.
type Issuer struct {
	CNPJ, MunicipalRegistration, CityCode, UF, Environment string
	CRT                                                    int
}

// IssuerReader lê o prestador (módulo issuer).
type IssuerReader interface {
	IssuerForNFSe(ctx context.Context, tenantID, issuerRef string) (Issuer, error)
}

// SignerProvider entrega o certificado A1.
type SignerProvider interface {
	SignerForNFSe(ctx context.Context, tenantID, issuerRef string) (*certificate.Certificate, error)
}

// NumberReserver reserva o número da DPS (modelo 99 no numbering).
type NumberReserver interface {
	ReserveDPS(ctx context.Context, tenantID, issuerRef, environment string, series int, documentRef string) (ref string, number int, err error)
	CloseDPS(ctx context.Context, tenantID, ref string) error
}

// Gateway fala com a Sefin Nacional.
type Gateway interface {
	Issue(ctx context.Context, cert *certificate.Certificate, environment int, signed []byte) (nfse.IssueResult, error)
	DPSKey(ctx context.Context, cert *certificate.Certificate, environment int, dpsID string) (string, bool, error)
	Get(ctx context.Context, cert *certificate.Certificate, environment int, key string) ([]byte, bool, error)
}

// Repository guarda as NFS-e.
type Repository interface {
	Create(ctx context.Context, s *entity.ServiceInvoice) error
	Update(ctx context.Context, s *entity.ServiceInvoice) error
	FindByIdempotencyKey(ctx context.Context, tenantID, key string) (*entity.ServiceInvoice, error)
	FindByKey(ctx context.Context, tenantID, key string) (*entity.ServiceInvoice, error)
	List(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.ServiceInvoice, int64, error)
}
