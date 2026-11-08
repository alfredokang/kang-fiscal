// Package http registra as rotas do módulo.
package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/adapter/http/handler"
)

// RegisterAdminRoutes pendura o certificado abaixo do emitente. PUT, e não POST: cada
// emitente tem um certificado ativo, e mandar outro substitui.
func RegisterAdminRoutes(r chi.Router, h *handler.CertificateHandler) {
	r.Put("/issuers/{id}/certificate", h.Upload)
	r.Get("/issuers/{id}/certificate", h.Get)
	r.Delete("/issuers/{id}/certificate", h.Revoke)
}
