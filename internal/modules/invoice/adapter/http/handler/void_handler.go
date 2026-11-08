package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

type voidRequest struct {
	IssuerID      string `json:"issuer_id"`
	Model         int    `json:"model"`
	Series        int    `json:"series"`
	From          int    `json:"from"`
	To            int    `json:"to"`
	Justification string `json:"justification"`
}

type voidResponse struct {
	ID            string    `json:"id"`
	Model         int       `json:"model"`
	Series        int       `json:"series"`
	From          int       `json:"from"`
	To            int       `json:"to"`
	Status        string    `json:"status"`
	CStat         int       `json:"cstat,omitempty"`
	Motive        string    `json:"motive,omitempty"`
	Protocol      string    `json:"protocol,omitempty"`
	Justification string    `json:"justification"`
	CreatedAt     time.Time `json:"created_at"`
}

func toVoidResponse(v *entity.Void) voidResponse {
	return voidResponse{ID: v.ID, Model: v.Model, Series: v.Series, From: v.From, To: v.To, Status: string(v.Status),
		CStat: v.CStat, Motive: v.Motive, Protocol: v.Protocol, Justification: v.Justification, CreatedAt: v.CreatedAt}
}

// VoidRange atende POST /v1/nfe/void-range: inutilização manual de uma faixa.
func (h *InvoiceHandler) VoidRange(w http.ResponseWriter, r *http.Request) {
	var body voidRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	if body.Model == 0 {
		body.Model = 55
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	v, err := h.svc.VoidRange(r.Context(), usecase.VoidCommand{TenantID: tenant, IssuerRef: body.IssuerID,
		Model: body.Model, Series: body.Series, From: body.From, To: body.To, Justification: body.Justification})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	status := http.StatusOK
	switch v.Status {
	case entity.EventRejected:
		status = http.StatusUnprocessableEntity
	case entity.EventUncertain, entity.EventSigned:
		status = http.StatusAccepted
	case entity.EventNotSent:
		status = http.StatusBadGateway
	}
	helpers.WriteJSON(w, status, toVoidResponse(v))
}

// Voids atende GET /v1/nfe/voids.
func (h *InvoiceHandler) Voids(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	page := pagination.Parse(r, "created_at")
	items, total, err := h.svc.Voids(r.Context(), tenant, page)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	out := make([]voidResponse, 0, len(items))
	for _, v := range items {
		out = append(out, toVoidResponse(v))
	}
	helpers.WriteJSON(w, http.StatusOK, pagination.NewPage(out, page, total))
}

// Sweep atende POST /v1/admin/void-sweep?older_than_hours=24: roda a varredura agora, a
// mesma do worker. Para quem não sobe o worker, ou quer fechar o mês antes do dia 10.
func (h *InvoiceHandler) Sweep(w http.ResponseWriter, r *http.Request) {
	hours, err := strconv.Atoi(r.URL.Query().Get("older_than_hours"))
	if err != nil || hours < 0 {
		hours = 24
	}
	res := h.svc.SweepPending(r.Context(), time.Duration(hours)*time.Hour, 500)
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"checked": res.Checked, "voided": res.Voided,
		"skipped": res.Skipped, "errors": res.Errors})
}
