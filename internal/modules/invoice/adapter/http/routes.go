// Package http registra as rotas da NF-e.
package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/adapter/http/handler"
)

// RegisterRoutes pendura as rotas da NF-e.
func RegisterRoutes(r chi.Router, h *handler.InvoiceHandler) {
	r.Post("/nfe", h.Issue)
	r.Get("/nfe", h.List)
	r.Get("/nfe/{key}", h.Get)
	r.Get("/nfe/{key}/xml", h.XML)
}
