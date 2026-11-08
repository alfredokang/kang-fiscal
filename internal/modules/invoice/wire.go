// Package invoice monta a emissão da NF-e.
package invoice

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/alfredokang/kang-fiscal/internal/modules/invoice/adapter/http"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/adapter/http/handler"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/infrastructure/gateway"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/infrastructure/persistence/postgres"
)

// Module expõe o serviço e as rotas.
type Module struct {
	Service *usecase.InvoiceService
	handler *handler.InvoiceHandler
}

// NewModule monta a árvore de dependências. As portas para os outros módulos vêm do
// composition root; Repo e Sefaz, se vazios, são os daqui (o teste troca a SEFAZ).
func NewModule(db *sql.DB, deps usecase.Deps) *Module {
	if deps.Repo == nil {
		deps.Repo = postgres.NewInvoiceRepository(db)
	}
	if deps.Events == nil {
		deps.Events = postgres.NewEventRepository(db)
	}
	if deps.Sefaz == nil {
		deps.Sefaz = gateway.NewSefazGateway()
	}
	svc := usecase.NewInvoiceService(deps)
	return &Module{Service: svc, handler: handler.NewInvoiceHandler(svc)}
}

// RegisterRoutes pendura as rotas do módulo.
func (m *Module) RegisterRoutes(r chi.Router) { adapterhttp.RegisterRoutes(r, m.handler) }
