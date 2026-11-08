// Package credential monta as credenciais de API por tenant.
package credential

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/alfredokang/kang-fiscal/internal/modules/credential/adapter/http"
	"github.com/alfredokang/kang-fiscal/internal/modules/credential/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/credential/infrastructure/persistence/postgres"
)

// Module expõe o serviço (o middleware de autenticação resolve tokens por ele) e as rotas.
type Module struct {
	Service *usecase.CredentialService
	handler *adapterhttp.Handler
}

// NewModule monta a árvore de dependências.
func NewModule(db *sql.DB, pepper string) *Module {
	svc := usecase.NewCredentialService(postgres.NewKeyRepository(db), pepper)
	return &Module{Service: svc, handler: adapterhttp.NewHandler(svc)}
}

// RegisterAdminRoutes pendura as rotas de administração.
func (m *Module) RegisterAdminRoutes(r chi.Router) { adapterhttp.RegisterAdminRoutes(r, m.handler) }
