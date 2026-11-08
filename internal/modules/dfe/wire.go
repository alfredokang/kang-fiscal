// Package dfe monta o recebimento de notas (distribuição de DF-e) e a manifestação.
package dfe

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/alfredokang/kang-fiscal/internal/modules/dfe/adapter/http"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/infrastructure/gateway"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/infrastructure/persistence/postgres"
)

// Module expõe o serviço (o worker sincroniza) e as rotas.
type Module struct {
	Service *usecase.DFeService
	handler *adapterhttp.Handler
}

// NewModule monta a árvore de dependências. gw nil = o gateway real.
func NewModule(db *sql.DB, issuers output.IssuerReader, signers output.SignerProvider, gw output.Gateway) *Module {
	if gw == nil {
		gw = gateway.New()
	}
	svc := usecase.NewDFeService(postgres.NewRepository(db), issuers, signers, gw)
	return &Module{Service: svc, handler: adapterhttp.NewHandler(svc)}
}

// RegisterRoutes pendura as rotas.
func (m *Module) RegisterRoutes(r chi.Router) { adapterhttp.RegisterRoutes(r, m.handler) }
