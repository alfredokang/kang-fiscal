// Package handler é a entrada HTTP da emissão — fina: lê, chama, traduz, escreve.
package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/adapter/http/request"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/adapter/http/response"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/service"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// InvoiceHandler atende as rotas da NF-e.
type InvoiceHandler struct{ svc *usecase.InvoiceService }

// NewInvoiceHandler monta o handler.
func NewInvoiceHandler(svc *usecase.InvoiceService) *InvoiceHandler { return &InvoiceHandler{svc: svc} }

// Issue atende POST /v1/nfe.
func (h *InvoiceHandler) Issue(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, helpers.MaxBodyBytes))
	if err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo grande demais ou ilegível", "INVALID_BODY")
		return
	}
	var body request.IssueRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido: "+err.Error(), "INVALID_BODY")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	inv, err := h.svc.Issue(r.Context(), usecase.IssueCommand{
		TenantID: tenant, IssuerRef: body.IssuerID, IdempotencyKey: r.Header.Get("Idempotency-Key"),
		RequestHash: entity.HashRequest(raw), Draft: toDraft(body),
	})
	if err != nil && inv == nil {
		writeDomainError(w, r, err)
		return
	}
	status := http.StatusCreated
	switch inv.Status {
	case entity.StatusRejected, entity.StatusDenied:
		status = http.StatusUnprocessableEntity
	case entity.StatusUncertain, entity.StatusSigned:
		status = http.StatusAccepted
	case entity.StatusNotSent:
		status = http.StatusBadGateway
	}
	helpers.WriteJSON(w, status, toResponse(inv))
}

// Get atende GET /v1/nfe/{key}. Com ?sync=sefaz, a nota incerta é consultada antes.
func (h *InvoiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	inv, err := h.svc.Get(r.Context(), tenant, chi.URLParam(r, "key"), r.URL.Query().Get("sync") == "sefaz")
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, toResponse(inv))
}

// XML atende GET /v1/nfe/{key}/xml: o nfeProc da nota autorizada, ou o XML assinado.
func (h *InvoiceHandler) XML(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	inv, err := h.svc.Get(r.Context(), tenant, chi.URLParam(r, "key"), false)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	doc, name := inv.SignedXML, inv.AccessKey+"-assinada.xml"
	if len(inv.AuthorizedXML) > 0 {
		doc, name = inv.AuthorizedXML, inv.AccessKey+"-procNFe.xml"
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write(doc)
}

// List atende GET /v1/nfe.
func (h *InvoiceHandler) List(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	page := pagination.Parse(r, "created_at")
	items, total, err := h.svc.List(r.Context(), tenant, page)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	out := make([]response.Invoice, 0, len(items))
	for _, i := range items {
		out = append(out, toResponse(i))
	}
	helpers.WriteJSON(w, http.StatusOK, pagination.NewPage(out, page, total))
}

func toDraft(b request.IssueRequest) service.Draft {
	addr := func(a request.Address) *service.Address {
		return &service.Address{Street: a.Street, Number: a.Number, Complement: a.Complement, District: a.District,
			CityCode: a.CityCode, CityName: a.CityName, UF: strings.ToUpper(a.UF), ZipCode: digits(a.ZipCode), Phone: digits(a.Phone)}
	}
	d := service.Draft{Model: b.Model, Series: b.Series, NatOp: b.NatureOfOp, Purpose: b.Purpose, Presence: b.Presence,
		FinalConsumer: b.FinalConsumer, AdditionalInfo: b.AdditionalInfo, FreightMode: 9,
		Recipient: &service.Recipient{CNPJ: digitsOrUpper(b.Recipient.CNPJ), CPF: digits(b.Recipient.CPF), Name: b.Recipient.Name,
			IE: digits(b.Recipient.IE), IEIndicator: b.Recipient.IEIndicator, Email: b.Recipient.Email}}
	if b.Model == 0 {
		d.Model = 55
	}
	if b.FreightMode != nil {
		d.FreightMode = *b.FreightMode
	}
	if d.Recipient.IEIndicator == 0 {
		d.Recipient.IEIndicator = 9
	}
	if b.Recipient.Address.UF != "" {
		d.Recipient.Address = addr(b.Recipient.Address)
	}
	for _, it := range b.Items {
		d.Lines = append(d.Lines, service.Line{Code: it.Code, Description: it.Description, NCM: digits(it.NCM),
			CEST: digits(it.CEST), GTIN: it.GTIN, Unit: it.Unit, Quantity: it.Quantity, UnitPrice: it.UnitPrice,
			Discount: it.Discount, Freight: it.Freight, Insurance: it.Insurance, Other: it.Other,
			Origin: it.Origin, Profile: it.TaxProfile})
	}
	for _, p := range b.Payments {
		d.Payments = append(d.Payments, service.Payment{Method: p.Method, Amount: p.Amount})
	}
	return d
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// digitsOrUpper mantém as letras do CNPJ alfanumérico.
func digitsOrUpper(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func toResponse(i *entity.Invoice) response.Invoice {
	out := response.Invoice{ID: i.ID, AccessKey: i.AccessKey, Model: i.Model, Series: i.Series, Number: i.Number,
		Environment: i.Environment, Status: string(i.Status), CStat: i.CStat, Motive: i.Motive, Protocol: i.Protocol,
		Total: i.TotalAmount, XMLHash: i.XMLHash, CreatedAt: i.CreatedAt, AuthorizedAt: i.AuthorizedAt}
	if i.CStat != 0 && !entity.IsAuthorization(i.CStat) {
		if c, ok := sefaz.Describe(i.CStat); ok {
			out.Hint = c.Hint
		}
	}
	return out
}

// writeDomainError é o ponto único de tradução erro → status do módulo (Art. 10).
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		helpers.WriteError(w, http.StatusNotFound, err.Error(), "INVOICE_NOT_FOUND")
	case errors.Is(err, domain.ErrIssuerNotFound):
		helpers.WriteError(w, http.StatusNotFound, err.Error(), "ISSUER_NOT_FOUND")
	case errors.Is(err, domain.ErrIdempotencyConflict):
		helpers.WriteError(w, http.StatusConflict, err.Error(), "IDEMPOTENCY_CONFLICT")
	case errors.Is(err, domain.ErrNotSupported):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "NOT_SUPPORTED")
	case errors.Is(err, domain.ErrTaxRateMissing):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "TAX_RATE_MISSING")
	case errors.Is(err, domain.ErrNoCertificate):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "NO_CERTIFICATE")
	case errors.Is(err, domain.ErrIdempotencyKey), errors.Is(err, domain.ErrRequiredField),
		errors.Is(err, domain.ErrInvalidRequest), errors.Is(err, domain.ErrIssuerIncomplete),
		errors.Is(err, domain.ErrTaxProfileNotFound), errors.Is(err, tax.ErrInvalidItem),
		errors.Is(err, tax.ErrRegime), errors.Is(err, tax.ErrUnsupportedCST):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "VALIDATION")
	default:
		helpers.WriteInternalError(w, r, err)
	}
}
