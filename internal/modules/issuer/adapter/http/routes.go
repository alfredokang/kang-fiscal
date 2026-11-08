// Package http registra as rotas do módulo.
package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/adapter/http/handler"
)

// RegisterAdminRoutes pendura as rotas de administração do emitente. O router recebido já
// passou pelo middleware de credencial de administração.
func RegisterAdminRoutes(r chi.Router, h *handler.IssuerHandler) {
	// Caminhos explícitos, e não r.Route("/issuers"): outros módulos penduram rotas
	// abaixo de /issuers/{id} (o certificado), e o chi não aceita dois Route no mesmo
	// prefixo.
	r.Get("/issuers", h.List)
	r.Post("/issuers", h.Register)
	r.Get("/issuers/{id}", h.Get)
	r.Put("/issuers/{id}", h.Update)
	// Rota própria, e não um campo do PUT: ir para produção é uma decisão com regra e sem
	// volta (Constituição, Art. 6.4).
	r.Post("/issuers/{id}/production", h.Promote)
}
