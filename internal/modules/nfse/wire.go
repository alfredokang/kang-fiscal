// Package nfse monta a emissão da NFS-e no padrão nacional.
package nfse

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/alfredokang/kang-fiscal/internal/modules/nfse/adapter/http"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/infrastructure/gateway"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/infrastructure/persistence/postgres"
)

// Module expõe o serviço e as rotas.
type Module struct {
	Service *usecase.Service
	handler *adapterhttp.Handler
}

// NewModule monta a árvore de dependências. gw nil = a Sefin real.
func NewModule(db *sql.DB, issuers output.IssuerReader, signers output.SignerProvider, numbers output.NumberReserver, gw output.Gateway) *Module {
	if gw == nil {
		gw = gateway.New()
	}
	svc := usecase.NewService(postgres.NewRepository(db), issuers, signers, numbers, gw)
	return &Module{Service: svc, handler: adapterhttp.NewHandler(svc)}
}

// RegisterRoutes pendura as rotas.
func (m *Module) RegisterRoutes(r chi.Router) { adapterhttp.RegisterRoutes(r, m.handler) }
