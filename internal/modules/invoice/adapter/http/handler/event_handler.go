package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/adapter/http/response"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
)

type cancelRequest struct {
	Justification string `json:"justification"`
}

type correctionRequest struct {
	// Text substitui todas as cartas anteriores: traga todas as correções da nota.
	Text string `json:"text"`
}

// Cancel atende POST /v1/nfe/{key}/cancel.
func (h *InvoiceHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	var body cancelRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	ev, err := h.svc.Cancel(r.Context(), tenant, chi.URLParam(r, "key"), body.Justification)
	writeEvent(w, r, ev, err)
}

// Correct atende POST /v1/nfe/{key}/correction.
func (h *InvoiceHandler) Correct(w http.ResponseWriter, r *http.Request) {
	var body correctionRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido", "INVALID_BODY")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	ev, err := h.svc.Correct(r.Context(), tenant, chi.URLParam(r, "key"), body.Text)
	writeEvent(w, r, ev, err)
}

// Events atende GET /v1/nfe/{key}/events.
func (h *InvoiceHandler) Events(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	events, err := h.svc.Events(r.Context(), tenant, chi.URLParam(r, "key"))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	out := make([]response.Event, 0, len(events))
	for _, e := range events {
		out = append(out, toEventResponse(e))
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

// EventXML atende GET /v1/nfe/events/{id}/xml: o procEventoNFe, ou o evento assinado.
func (h *InvoiceHandler) EventXML(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	e, err := h.svc.Event(r.Context(), tenant, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	doc, name := e.SignedXML, e.AccessKey+"-"+e.Type+"-assinado.xml"
	if len(e.ProcXML) > 0 {
		doc, name = e.ProcXML, e.AccessKey+"-"+e.Type+"-procEventoNFe.xml"
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write(doc)
}

func writeEvent(w http.ResponseWriter, r *http.Request, e *entity.Event, err error) {
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	status := http.StatusOK
	switch e.Status {
	case entity.EventRejected:
		status = http.StatusUnprocessableEntity
	case entity.EventUncertain, entity.EventSigned:
		status = http.StatusAccepted
	case entity.EventNotSent:
		status = http.StatusBadGateway
	}
	helpers.WriteJSON(w, status, toEventResponse(e))
}

func toEventResponse(e *entity.Event) response.Event {
	kind := "correction"
	switch e.Type {
	case nfe.EventCancellation:
		kind = "cancellation"
	case nfe.EventSubstitution:
		kind = "substitution" // a NFC-e online coberta pela offline (110112)
	}
	out := response.Event{ID: e.ID, Type: kind, Code: e.Type, Seq: e.Seq, Status: string(e.Status), CStat: e.CStat,
		Motive: e.Motive, Protocol: e.Protocol, Text: e.Text, CreatedAt: e.CreatedAt}
	if e.Status == entity.EventRejected {
		if c, ok := sefaz.Describe(e.CStat); ok {
			out.Hint = c.Hint
		}
	}
	return out
}
