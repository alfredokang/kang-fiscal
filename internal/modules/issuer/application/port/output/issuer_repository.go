// Package output declara o que o módulo issuer precisa do mundo de fora.
package output

import (
	"context"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/entity"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/valueobject"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// IssuerRepository persiste o emitente. Toda leitura recebe o tenant: o repositório nunca
// devolve emitente de outro tenant, nem por id (Constituição, Art. 8).
type IssuerRepository interface {
	// Create grava um emitente novo. CNPJ repetido no mesmo tenant devolve
	// domain.ErrIssuerAlreadyExists.
	Create(ctx context.Context, issuer *entity.Issuer) error
	Update(ctx context.Context, issuer *entity.Issuer) error
	FindByID(ctx context.Context, tenantID string, id vo.IssuerID) (*entity.Issuer, error)
	List(ctx context.Context, tenantID string, p pagination.Params) ([]*entity.Issuer, int64, error)
}
