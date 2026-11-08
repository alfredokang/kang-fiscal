// Package output declara o que o recebimento de notas precisa do mundo de fora.
package output

import (
	"context"

	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// Issuer é o que este módulo precisa do emitente (aqui, o destinatário das notas).
type Issuer struct {
	CNPJ, UF, Environment string
}

// IssuerReader lê o emitente, que é de outro módulo.
type IssuerReader interface {
	IssuerForDFe(ctx context.Context, tenantID, issuerRef string) (Issuer, error)
}

// SignerProvider entrega o certificado A1 do emitente.
type SignerProvider interface {
	SignerForDFe(ctx context.Context, tenantID, issuerRef string) (*certificate.Certificate, error)
}

// Gateway fala com o Ambiente Nacional.
type Gateway interface {
	Distribute(ctx context.Context, cert *certificate.Certificate, q sefaz.DistributionQuery) (sefaz.DistributionResult, error)
	SendEvent(ctx context.Context, cert *certificate.Certificate, t sefaz.Target, batchID int64, signed []byte) (sefaz.EventResult, error)
}

// Repository guarda o cursor e os documentos.
type Repository interface {
	Cursor(ctx context.Context, tenantID, issuerRef string) (entity.Cursor, error)
	SaveCursor(ctx context.Context, c entity.Cursor) error
	// DueCursors lista os cursores cuja próxima consulta já pode acontecer.
	DueCursors(ctx context.Context, limit int) ([]entity.Cursor, error)
	// SaveDocument grava o documento; o mesmo NSU do mesmo emitente é ignorado.
	SaveDocument(ctx context.Context, d *entity.Document) error
	Find(ctx context.Context, tenantID, id string) (*entity.Document, error)
	List(ctx context.Context, tenantID, issuerRef string, page pagination.Params) ([]*entity.Document, int64, error)
	SetManifestation(ctx context.Context, tenantID, accessKey, tpEvento string) error
}
