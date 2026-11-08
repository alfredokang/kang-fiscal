// Package http é a entrada HTTP da numeração: só o ajuste do próximo número. Reservar é
// interno — quem reserva é a emissão.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
)

// Handler atende a rota da numeração.
type Handler struct{ svc *usecase.NumberingService }

// NewHandler monta o handler.
func NewHandler(svc *usecase.NumberingService) *Handler { return &Handler{svc: svc} }

type setNextRequest struct {
	Environment string `json:"environment"`
	NextNumber  int    `json:"next_number"`
}

// SetNext atende PUT /v1/admin/issuers/{id}/series/{model}/{series}.
//
// Para quem migra de outro emissor: a numeração continua de onde parou, sem buraco nem
// repetição.
func (h *Handler) SetNext(w http.ResponseWriter, r *http.Request) {
	var body setNextRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	model, err1 := strconv.Atoi(chi.URLParam(r, "model"))
	series, err2 := strconv.Atoi(chi.URLParam(r, "series"))
	if err1 != nil || err2 != nil {
		helpers.WriteError(w, http.StatusUnprocessableEntity, "modelo e série são números", "VALIDATION")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	key := entity.SeriesKey{TenantID: tenant, IssuerRef: chi.URLParam(r, "id"), Model: model, Series: series, Environment: body.Environment}
	if err := h.svc.SetNext(r.Context(), key, body.NextNumber); err != nil {
		switch {
		case errors.Is(err, domain.ErrIssuerNotFound):
			helpers.WriteError(w, http.StatusNotFound, err.Error(), "ISSUER_NOT_FOUND")
		case errors.Is(err, domain.ErrNumberBackward):
			helpers.WriteError(w, http.StatusConflict, err.Error(), "NUMBER_BACKWARD")
		case errors.Is(err, domain.ErrInvalidModel), errors.Is(err, domain.ErrInvalidSeries),
			errors.Is(err, domain.ErrInvalidNumber), errors.Is(err, domain.ErrRequiredField):
			helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "VALIDATION")
		default:
			helpers.WriteInternalError(w, r, err)
		}
		return
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"model": model, "series": series,
		"environment": body.Environment, "next_number": body.NextNumber})
}

// RegisterAdminRoutes pendura a rota.
func RegisterAdminRoutes(r chi.Router, h *Handler) {
	r.Put("/issuers/{id}/series/{model}/{series}", h.SetNext)
}
