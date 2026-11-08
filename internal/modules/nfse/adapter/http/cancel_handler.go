package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
)

type cancelRequest struct {
	Reason        int    `json:"reason"` // 1 erro na emissão, 2 serviço não prestado, 9 outros
	Justification string `json:"justification"`
	// Mode "direct" (padrão) cancela na hora; "analysis" pede a análise fiscal do município.
	Mode string `json:"mode"`
}

type eventResponse struct {
	ID            string    `json:"id"`
	Kind          string    `json:"type"`
	Reason        int       `json:"reason"`
	Justification string    `json:"justification"`
	Status        string    `json:"status"`
	ErrorCode     string    `json:"error_code,omitempty"`
	Message       string    `json:"message,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toEvent(e *entity.Event) eventResponse {
	return eventResponse{ID: e.ID, Kind: e.Kind, Reason: e.Reason, Justification: e.Justification, Status: string(e.Status),
		ErrorCode: e.ErrorCode, Message: e.Message, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
}

// Cancel atende POST /v1/nfse/{key}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	var b cancelRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, helpers.MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido: "+err.Error(), "INVALID_BODY")
		return
	}
	if b.Mode != "" && b.Mode != "direct" && b.Mode != "analysis" {
		helpers.WriteError(w, http.StatusUnprocessableEntity, `mode: "direct" ou "analysis"`, "VALIDATION")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	ev, err := h.svc.Cancel(r.Context(), usecase.CancelCommand{TenantID: tenant, Key: chi.URLParam(r, "key"),
		Reason: b.Reason, Justification: b.Justification, Analysis: b.Mode == "analysis"})
	if err != nil {
		writeError(w, r, err)
		return
	}
	status := http.StatusOK
	switch ev.Status {
	case entity.EventRejected:
		status = http.StatusUnprocessableEntity
	case entity.EventUncertain, entity.EventSigned:
		status = http.StatusAccepted
	case entity.EventNotSent:
		status = http.StatusBadGateway
	}
	helpers.WriteJSON(w, status, toEvent(ev))
}

// Events atende GET /v1/nfse/{key}/events.
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	items, err := h.svc.Events(r.Context(), tenant, chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]eventResponse, 0, len(items))
	for _, e := range items {
		out = append(out, toEvent(e))
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}
