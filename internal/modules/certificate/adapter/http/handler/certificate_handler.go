// Package handler é a entrada HTTP do módulo — fina: lê, chama, traduz, escreve.
package handler

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/adapter/http/request"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/adapter/http/response"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/port/input"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
)

// CertificateHandler atende as rotas do certificado.
type CertificateHandler struct {
	service input.CertificateAdmin
}

// NewCertificateHandler monta o handler.
func NewCertificateHandler(s input.CertificateAdmin) *CertificateHandler {
	return &CertificateHandler{service: s}
}

// Upload atende PUT /v1/admin/issuers/{id}/certificate.
func (h *CertificateHandler) Upload(w http.ResponseWriter, r *http.Request) {
	var body request.UploadCertificateRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	pfx, err := base64.StdEncoding.DecodeString(strings.TrimSpace(body.PFXBase64))
	if err != nil {
		helpers.WriteError(w, http.StatusUnprocessableEntity, "pfx_base64 não é base64 válido", "VALIDATION")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	out, err := h.service.Upload(r.Context(), command.UploadCertificate{
		TenantID: tenant, IssuerRef: chi.URLParam(r, "id"), PFX: pfx, Password: body.Password,
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, toResponse(out))
}

// Get atende GET /v1/admin/issuers/{id}/certificate.
func (h *CertificateHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	out, err := h.service.Get(r.Context(), command.IssuerCertificate{TenantID: tenant, IssuerRef: chi.URLParam(r, "id")})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, toResponse(out))
}

// Revoke atende DELETE /v1/admin/issuers/{id}/certificate.
func (h *CertificateHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	if err := h.service.Revoke(r.Context(), command.IssuerCertificate{TenantID: tenant, IssuerRef: chi.URLParam(r, "id")}); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toResponse(d dto.CertificateDTO) response.CertificateResponse {
	return response.CertificateResponse{
		IssuerID: d.IssuerRef, HolderName: d.HolderName, CNPJ: d.CNPJ, CAName: d.CAName,
		Serial: d.Serial, Fingerprint: d.Fingerprint, NotBefore: d.NotBefore, NotAfter: d.NotAfter,
		DaysLeft: d.DaysLeft, ExpiringSoon: d.ExpiringSoon, CreatedAt: d.CreatedAt,
	}
}

// writeDomainError é o ponto único de tradução erro → status do módulo (Art. 10).
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrIssuerNotFound), errors.Is(err, domain.ErrInvalidID):
		helpers.WriteError(w, http.StatusNotFound, domain.ErrIssuerNotFound.Error(), "ISSUER_NOT_FOUND")
	case errors.Is(err, domain.ErrNotFound):
		helpers.WriteError(w, http.StatusNotFound, err.Error(), "CERTIFICATE_NOT_FOUND")
	case errors.Is(err, domain.ErrEncryptionDisabled):
		helpers.WriteError(w, http.StatusServiceUnavailable, err.Error(), "ENCRYPTION_DISABLED")
	case errors.Is(err, domain.ErrEmptyFile), errors.Is(err, domain.ErrFileTooLarge),
		errors.Is(err, domain.ErrWrongPassword), errors.Is(err, domain.ErrNotRSA),
		errors.Is(err, domain.ErrExpired), errors.Is(err, domain.ErrNoCNPJ),
		errors.Is(err, domain.ErrCNPJMismatch):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "INVALID_CERTIFICATE")
	default:
		helpers.WriteInternalError(w, r, err)
	}
}
