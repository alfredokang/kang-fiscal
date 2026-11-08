// Package taxrule monta o motor tributário: perfis, alíquotas por vigência e simulação.
package taxrule

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/alfredokang/kang-fiscal/internal/modules/taxrule/adapter/http"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/adapter/http/handler"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/infrastructure/persistence/postgres"
)

// Module expõe o serviço (a emissão vai pedir perfil e alíquota por porta) e as rotas.
type Module struct {
	Service *usecase.TaxRuleService
	handler *handler.TaxRuleHandler
}

// NewModule monta a árvore de dependências.
func NewModule(db *sql.DB) *Module {
	svc := usecase.NewTaxRuleService(postgres.NewProfileRepository(db), postgres.NewRateRepository(db))
	return &Module{Service: svc, handler: handler.NewTaxRuleHandler(svc)}
}

// RegisterRoutes pendura as rotas do módulo.
func (m *Module) RegisterRoutes(admin chi.Router) { adapterhttp.RegisterAdminRoutes(admin, m.handler) }
