// Package input declara o que o módulo issuer oferece.
package input

import (
	"context"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/query"
)

// IssuerCommands são as escritas.
type IssuerCommands interface {
	Register(ctx context.Context, cmd command.RegisterIssuer) (dto.IssuerDTO, error)
	Update(ctx context.Context, cmd command.UpdateIssuer) (dto.IssuerDTO, error)
	Promote(ctx context.Context, cmd command.PromoteIssuer) (dto.IssuerDTO, error)
}

// IssuerQueries são as leituras.
type IssuerQueries interface {
	Get(ctx context.Context, q query.GetIssuer) (dto.IssuerDTO, error)
	List(ctx context.Context, q query.ListIssuers) ([]dto.IssuerDTO, int64, error)
}
