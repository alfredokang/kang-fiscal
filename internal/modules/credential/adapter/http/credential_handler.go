// Package http é a administração das credenciais (só com o ADMIN_TOKEN).
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/credential/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
)

// Handler atende as rotas das credenciais.
type Handler struct{ svc *usecase.CredentialService }

// NewHandler monta o handler.
func NewHandler(svc *usecase.CredentialService) *Handler { return &Handler{svc: svc} }

type createRequest struct {
	TenantID string   `json:"tenant_id"`
	Name     string   `json:"name"`
	Scopes   []string `json:"scopes"`
}

type keyResponse struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	// Token só vem na criação. Guarde: ele não é mostrado de novo.
	Token string `json:"token,omitempty"`
}

func toResponse(k *entity.Key, token string) keyResponse {
	return keyResponse{ID: k.ID, TenantID: k.TenantID, Name: k.Name, Prefix: k.Prefix, Scopes: k.Scopes,
		CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt, RevokedAt: k.RevokedAt, Token: token}
}

// Create atende POST /v1/admin/credentials.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body createRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	k, token, err := h.svc.Create(r.Context(), body.TenantID, body.Name, body.Scopes)
	if err != nil {
		writeError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusCreated, toResponse(k, token))
}

// List atende GET /v1/admin/credentials?tenant_id=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.svc.List(r.Context(), r.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]keyResponse, 0, len(keys))
	for _, k := range keys {
		out = append(out, toResponse(k, ""))
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

// Revoke atende DELETE /v1/admin/credentials/{id}.
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Revoke(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		helpers.WriteError(w, http.StatusNotFound, err.Error(), "CREDENTIAL_NOT_FOUND")
	case errors.Is(err, domain.ErrNoPepper):
		helpers.WriteError(w, http.StatusServiceUnavailable, err.Error(), "PEPPER_MISSING")
	case errors.Is(err, domain.ErrInvalidTenant), errors.Is(err, domain.ErrInvalidScope), errors.Is(err, domain.ErrRequiredField):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "VALIDATION")
	default:
		helpers.WriteInternalError(w, r, err)
	}
}

// RegisterAdminRoutes pendura as rotas.
func RegisterAdminRoutes(r chi.Router, h *Handler) {
	r.Post("/credentials", h.Create)
	r.Get("/credentials", h.List)
	r.Delete("/credentials/{id}", h.Revoke)
}
