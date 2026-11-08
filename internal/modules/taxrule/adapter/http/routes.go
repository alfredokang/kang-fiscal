// Package http registra as rotas do módulo.
package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/adapter/http/handler"
)

// RegisterAdminRoutes pendura as rotas do motor tributário.
func RegisterAdminRoutes(r chi.Router, h *handler.TaxRuleHandler) {
	r.Get("/tax-profiles", h.ListProfiles)
	r.Get("/tax-profiles/{code}", h.GetProfile)
	// PUT com o código na URL: o código é escolhido por quem cadastra ("venda-cafe-sp") e é
	// por ele que o produto aponta para o perfil.
	r.Put("/tax-profiles/{code}", h.SaveProfile)
	r.Get("/tax-rates", h.ListRates)
	r.Post("/tax-rates", h.AddRate)
	r.Post("/tax/simulate", h.Simulate)
}

// RegisterTenantRoutes pendura o que uma credencial de tenant alcança: os próprios perfis,
// a simulação e a leitura das alíquotas. Cadastrar alíquota é só do administrador — é lei,
// vale para todos os tenants da instalação.
func RegisterTenantRoutes(r chi.Router, h *handler.TaxRuleHandler) {
	r.Get("/tax-profiles", h.ListProfiles)
	r.Get("/tax-profiles/{code}", h.GetProfile)
	r.Put("/tax-profiles/{code}", h.SaveProfile)
	r.Get("/tax-rates", h.ListRates)
	r.Post("/tax/simulate", h.Simulate)
}
