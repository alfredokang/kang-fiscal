// Package handler é a entrada HTTP do módulo — fina: lê, chama, traduz, escreve.
package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/adapter/http/mapper"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/adapter/http/request"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/adapter/http/response"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/port/input"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/query"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// IssuerHandler atende as rotas do emitente.
type IssuerHandler struct {
	commands   input.IssuerCommands
	queries    input.IssuerQueries
	sortFields []string
}

// NewIssuerHandler monta o handler.
func NewIssuerHandler(c input.IssuerCommands, q input.IssuerQueries, sortFields []string) *IssuerHandler {
	return &IssuerHandler{commands: c, queries: q, sortFields: sortFields}
}

// Register atende POST /v1/admin/issuers.
func (h *IssuerHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body request.RegisterIssuerRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	out, err := h.commands.Register(r.Context(), command.RegisterIssuer{
		TenantID: tenant, CNPJ: body.CNPJ, Data: mapper.ToFiscalData(body.FiscalDataRequest),
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusCreated, mapper.ToResponse(out))
}

// Update atende PUT /v1/admin/issuers/{id}.
func (h *IssuerHandler) Update(w http.ResponseWriter, r *http.Request) {
	var body request.FiscalDataRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	out, err := h.commands.Update(r.Context(), command.UpdateIssuer{
		TenantID: tenant, IssuerID: chi.URLParam(r, "id"), Data: mapper.ToFiscalData(body),
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, mapper.ToResponse(out))
}

// Promote atende POST /v1/admin/issuers/{id}/production.
func (h *IssuerHandler) Promote(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	out, err := h.commands.Promote(r.Context(), command.PromoteIssuer{
		TenantID: tenant, IssuerID: chi.URLParam(r, "id"),
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, mapper.ToResponse(out))
}

// Get atende GET /v1/admin/issuers/{id}.
func (h *IssuerHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	out, err := h.queries.Get(r.Context(), query.GetIssuer{TenantID: tenant, IssuerID: chi.URLParam(r, "id")})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, mapper.ToResponse(out))
}

// List atende GET /v1/admin/issuers.
func (h *IssuerHandler) List(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	page := pagination.Parse(r, h.sortFields...)
	items, total, err := h.queries.List(r.Context(), query.ListIssuers{TenantID: tenant, Page: page})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	out := make([]response.IssuerResponse, 0, len(items))
	for _, d := range items {
		out = append(out, mapper.ToResponse(d))
	}
	helpers.WriteJSON(w, http.StatusOK, pagination.NewPage(out, page, total))
}

// writeDomainError é o ponto único de tradução erro → status do módulo (Art. 10).
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrIssuerNotFound), errors.Is(err, domain.ErrInvalidIssuerID):
		helpers.WriteError(w, http.StatusNotFound, domain.ErrIssuerNotFound.Error(), "ISSUER_NOT_FOUND")
	case errors.Is(err, domain.ErrIssuerAlreadyExists):
		helpers.WriteError(w, http.StatusConflict, err.Error(), "ISSUER_ALREADY_EXISTS")
	case errors.Is(err, domain.ErrIncompleteRegistration), errors.Is(err, domain.ErrAlreadyInProduction):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "ISSUER_STATE")
	case errors.Is(err, domain.ErrInvalidCNPJ), errors.Is(err, domain.ErrInvalidUF),
		errors.Is(err, domain.ErrInvalidCityCode), errors.Is(err, domain.ErrCityOutsideUF),
		errors.Is(err, domain.ErrInvalidZipCode), errors.Is(err, domain.ErrInvalidTaxRegime),
		errors.Is(err, domain.ErrInvalidStateReg), errors.Is(err, domain.ErrInvalidEnvironment),
		errors.Is(err, domain.ErrRequiredField):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "VALIDATION")
	default:
		helpers.WriteInternalError(w, r, err)
	}
}
