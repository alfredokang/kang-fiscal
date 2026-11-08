// Package http é a entrada HTTP do recebimento de notas.
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// Handler atende as rotas.
type Handler struct{ svc *usecase.DFeService }

// NewHandler monta o handler.
func NewHandler(svc *usecase.DFeService) *Handler { return &Handler{svc: svc} }

type docResponse struct {
	ID            string    `json:"id"`
	NSU           int64     `json:"nsu"`
	Kind          string    `json:"kind"`
	AccessKey     string    `json:"access_key"`
	EmitterDoc    string    `json:"emitter_document"`
	EmitterName   string    `json:"emitter_name"`
	Total         string    `json:"total"`
	IssuedAt      string    `json:"issued_at"`
	Situation     string    `json:"situation,omitempty"`
	Manifestation string    `json:"manifestation,omitempty"`
	ReceivedAt    time.Time `json:"received_at"`
}

func toDoc(d *entity.Document) docResponse {
	return docResponse{ID: d.ID, NSU: d.NSU, Kind: string(d.Kind), AccessKey: d.AccessKey, EmitterDoc: d.EmitterDoc,
		EmitterName: d.EmitterName, Total: d.Total, IssuedAt: d.IssuedAt, Situation: d.Situation,
		Manifestation: d.Manifestation, ReceivedAt: d.CreatedAt}
}

// Sync atende POST /v1/dfe/sync?issuer_id=.
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	res, err := h.svc.Sync(r.Context(), tenant, r.URL.Query().Get("issuer_id"))
	if errors.Is(err, domain.ErrTooSoon) {
		helpers.WriteJSON(w, http.StatusTooManyRequests, map[string]any{"error": err.Error(), "code": "TOO_SOON",
			"next_query_at": res.NextQueryAt})
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"received": res.Received, "cstat": res.CStat,
		"motive": res.Motive, "next_query_at": res.NextQueryAt})
}

// List atende GET /v1/dfe/documents?issuer_id=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	page := pagination.Parse(r, "nsu")
	items, total, err := h.svc.List(r.Context(), tenant, r.URL.Query().Get("issuer_id"), page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]docResponse, 0, len(items))
	for _, d := range items {
		out = append(out, toDoc(d))
	}
	helpers.WriteJSON(w, http.StatusOK, pagination.NewPage(out, page, total))
}

// XML atende GET /v1/dfe/documents/{id}/xml.
func (h *Handler) XML(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	d, err := h.svc.Get(r.Context(), tenant, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+d.AccessKey+"-"+string(d.Kind)+`.xml"`)
	_, _ = w.Write(d.XML)
}

var manifestTypes = map[string]string{
	"confirmation": nfe.ManifestConfirmation, "awareness": nfe.ManifestAwareness,
	"unknown": nfe.ManifestUnknown, "not_performed": nfe.ManifestNotPerformed,
}

type manifestRequest struct {
	// Type: awareness (ciência — libera o XML completo), confirmation (a mercadoria
	// chegou), unknown (não reconhece a nota) ou not_performed (exige justificativa).
	Type          string `json:"type"`
	Justification string `json:"justification"`
}

// Manifest atende POST /v1/dfe/documents/{id}/manifest.
func (h *Handler) Manifest(w http.ResponseWriter, r *http.Request) {
	var body manifestRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	tp, ok := manifestTypes[body.Type]
	if !ok {
		helpers.WriteError(w, http.StatusUnprocessableEntity, "type: awareness, confirmation, unknown ou not_performed", "VALIDATION")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	res, err := h.svc.Manifest(r.Context(), tenant, chi.URLParam(r, "id"), tp, body.Justification)
	if err != nil {
		writeError(w, r, err)
		return
	}
	status := http.StatusOK
	if !res.Registered() && res.CStat != 573 {
		status = http.StatusUnprocessableEntity
	}
	out := map[string]any{"cstat": res.CStat, "motive": res.Motive, "protocol": res.Protocol}
	if c, ok := sefaz.Describe(res.CStat); ok && status != http.StatusOK {
		out["hint"] = c.Hint
	}
	helpers.WriteJSON(w, status, out)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrIssuerNotFound):
		helpers.WriteError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
	case errors.Is(err, domain.ErrNoCertificate):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "NO_CERTIFICATE")
	case errors.Is(err, domain.ErrInvalidRequest), errors.Is(err, domain.ErrNotSummary):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "VALIDATION")
	case errors.Is(err, sefaz.ErrUncertain), errors.Is(err, sefaz.ErrCertificateRejected), errors.Is(err, sefaz.ErrServerUntrusted):
		helpers.WriteError(w, http.StatusBadGateway, err.Error(), "SEFAZ_UNAVAILABLE")
	default:
		helpers.WriteInternalError(w, r, err)
	}
}

// RegisterRoutes pendura as rotas.
func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/dfe/sync", h.Sync)
	r.Get("/dfe/documents", h.List)
	r.Get("/dfe/documents/{id}/xml", h.XML)
	r.Post("/dfe/documents/{id}/manifest", h.Manifest)
}
