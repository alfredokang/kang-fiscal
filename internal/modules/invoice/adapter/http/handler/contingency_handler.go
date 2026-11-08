package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
)

type contingencyRequest struct {
	UF          string `json:"uf"`
	Model       int    `json:"model"`
	Environment string `json:"environment"`
	Reason      string `json:"reason"`
}

type contingencyResponse struct {
	UF          string    `json:"uf"`
	Model       int       `json:"model"`
	Environment string    `json:"environment"`
	Kind        string    `json:"kind"`
	Reason      string    `json:"reason"`
	StartedAt   time.Time `json:"started_at"`
}

func toContingency(c entity.Contingency) contingencyResponse {
	return contingencyResponse{UF: c.UF, Model: c.Model, Environment: c.Environment, Kind: c.Kind, Reason: c.Reason, StartedAt: c.StartedAt}
}

// Contingencies atende GET /v1/admin/contingency.
func (h *InvoiceHandler) Contingencies(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Contingencies(r.Context())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	out := make([]contingencyResponse, 0, len(list))
	for _, c := range list {
		out = append(out, toContingency(c))
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

// ActivateContingency atende POST /v1/admin/contingency.
func (h *InvoiceHandler) ActivateContingency(w http.ResponseWriter, r *http.Request) {
	var body contingencyRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	if body.Model == 0 {
		body.Model = 55
	}
	if body.Environment == "" {
		body.Environment = "homologation"
	}
	c, err := h.svc.ActivateContingency(r.Context(), body.UF, body.Model, body.Environment, body.Reason)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, toContingency(c))
}

// DeactivateContingency atende DELETE /v1/admin/contingency/{uf}?model=55&environment=homologation.
func (h *InvoiceHandler) DeactivateContingency(w http.ResponseWriter, r *http.Request) {
	model, _ := strconv.Atoi(r.URL.Query().Get("model"))
	if model == 0 {
		model = 55
	}
	env := r.URL.Query().Get("environment")
	if env == "" {
		env = "homologation"
	}
	uf := strings.ToUpper(strings.TrimPrefix(r.URL.Path[strings.LastIndex(r.URL.Path, "/"):], "/"))
	if err := h.svc.DeactivateContingency(r.Context(), uf, model, env); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
