// Package numbering monta a numeração das notas.
package numbering

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/alfredokang/kang-fiscal/internal/modules/numbering/adapter/http"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/infrastructure/persistence/postgres"
)

// Module expõe o serviço (a emissão reserva por porta) e a rota de ajuste.
type Module struct {
	Service *usecase.NumberingService
	handler *adapterhttp.Handler
}

// NewModule monta a árvore de dependências.
func NewModule(db *sql.DB, issuers output.IssuerChecker) *Module {
	svc := usecase.NewNumberingService(postgres.NewSeriesRepository(db), issuers)
	return &Module{Service: svc, handler: adapterhttp.NewHandler(svc)}
}

// RegisterRoutes pendura as rotas do módulo.
func (m *Module) RegisterRoutes(admin chi.Router) { adapterhttp.RegisterAdminRoutes(admin, m.handler) }
