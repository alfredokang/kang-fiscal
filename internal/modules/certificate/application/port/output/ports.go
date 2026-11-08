// Package output declara o que o módulo de certificado precisa do mundo de fora.
package output

import (
	"context"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/entity"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/valueobject"
)

// Sealed é o .pfx e a senha já cifrados. O tipo existe para que o repositório nunca
// receba o segredo em claro — ele não tem como gravar o que não recebeu.
type Sealed struct {
	PFX      string
	Password string
}

// CertificateRepository guarda o certificado ativo de cada emitente.
type CertificateRepository interface {
	// Replace grava o novo certificado como o ativo do emitente e, na mesma transação,
	// APAGA o segredo do anterior (os dados públicos ficam, para o histórico). Certificado
	// trocado guardado é risco sem uso nenhum.
	Replace(ctx context.Context, cert *entity.Certificate, sealed Sealed) error
	FindActive(ctx context.Context, tenantID string, issuer vo.IssuerRef) (*entity.Certificate, error)
	FindActiveSealed(ctx context.Context, tenantID string, issuer vo.IssuerRef) (*entity.Certificate, Sealed, error)
	// Revoke desativa o ativo e apaga o segredo.
	Revoke(ctx context.Context, tenantID string, issuer vo.IssuerRef, at time.Time) error
}

// SecretBox cifra e decifra. É o shared/secretbox, visto daqui só pelo que o módulo usa.
type SecretBox interface {
	Ready() bool
	Seal(plain string) (string, error)
	Open(sealed string) (string, error)
}

// IssuerSnapshot é o que este módulo precisa do emitente: só o CNPJ.
type IssuerSnapshot struct {
	CNPJ string
}

// IssuerReader lê o emitente, que é de outro módulo (Art. 1.2). Emitente de outro tenant
// é "não encontrado".
type IssuerReader interface {
	IssuerForCertificate(ctx context.Context, tenantID, issuerRef string) (IssuerSnapshot, error)
}
