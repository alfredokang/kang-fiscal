// Package input declara o que o módulo de certificado oferece.
package input

import (
	"context"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/dto"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
)

// CertificateAdmin é o que a rota de administração usa.
type CertificateAdmin interface {
	Upload(ctx context.Context, cmd command.UploadCertificate) (dto.CertificateDTO, error)
	Get(ctx context.Context, q command.IssuerCertificate) (dto.CertificateDTO, error)
	Revoke(ctx context.Context, cmd command.IssuerCertificate) error
}

// SignerProvider entrega o certificado aberto para assinar. Só a emissão usa, e nunca por
// HTTP: o certificado decifrado existe só em memória, só durante a assinatura.
type SignerProvider interface {
	Signer(ctx context.Context, q command.IssuerCertificate) (*certificate.Certificate, error)
}
