// Package issuer monta o módulo do emitente.
package issuer

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/alfredokang/kang-fiscal/internal/modules/issuer/adapter/http"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/adapter/http/handler"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/infrastructure/persistence/postgres"
)

// Module expõe o que o composition root precisa. O IssuerService é público porque os
// próximos módulos (certificado, nota) vão ler o emitente por porta, e o adapter em
// shared/server/ o embrulha.
type Module struct {
	IssuerService *usecase.IssuerService
	handler       *handler.IssuerHandler
}

// NewModule monta a árvore de dependências.
func NewModule(db *sql.DB) *Module {
	repo := postgres.NewIssuerRepository(db)
	service := usecase.NewIssuerService(repo)
	return &Module{
		IssuerService: service,
		handler:       handler.NewIssuerHandler(service, service, postgres.SortFields),
	}
}

// RegisterRoutes pendura as rotas do módulo.
func (m *Module) RegisterRoutes(admin chi.Router) {
	adapterhttp.RegisterAdminRoutes(admin, m.handler)
}
