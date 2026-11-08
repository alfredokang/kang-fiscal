// Package handler é a entrada HTTP do módulo — fina: lê, chama, traduz, escreve.
package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/adapter/http/request"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/adapter/http/response"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// TaxRuleHandler atende as rotas do motor tributário.
type TaxRuleHandler struct {
	svc *usecase.TaxRuleService
}

// NewTaxRuleHandler monta o handler.
func NewTaxRuleHandler(svc *usecase.TaxRuleService) *TaxRuleHandler { return &TaxRuleHandler{svc: svc} }

// SaveProfile atende PUT /v1/admin/tax-profiles/{code}.
func (h *TaxRuleHandler) SaveProfile(w http.ResponseWriter, r *http.Request) {
	var body request.ProfileRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido: "+err.Error(), "INVALID_BODY")
		return
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	p, err := h.svc.SaveProfile(r.Context(), tenant, chi.URLParam(r, "code"), body.Name, toRules(body.Rules))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, toProfileResponse(p))
}

// GetProfile atende GET /v1/admin/tax-profiles/{code}.
func (h *TaxRuleHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	p, err := h.svc.GetProfile(r.Context(), tenant, chi.URLParam(r, "code"))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, toProfileResponse(p))
}

// ListProfiles atende GET /v1/admin/tax-profiles.
func (h *TaxRuleHandler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	page := pagination.Parse(r, "code")
	items, total, err := h.svc.ListProfiles(r.Context(), tenant, page)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	out := make([]response.Profile, 0, len(items))
	for _, p := range items {
		out = append(out, toProfileResponse(p))
	}
	helpers.WriteJSON(w, http.StatusOK, pagination.NewPage(out, page, total))
}

// AddRate atende POST /v1/admin/tax-rates.
func (h *TaxRuleHandler) AddRate(w http.ResponseWriter, r *http.Request) {
	var body request.RateRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido: "+err.Error(), "INVALID_BODY")
		return
	}
	from, err := time.Parse("2006-01-02", body.ValidFrom)
	if err != nil {
		helpers.WriteError(w, http.StatusUnprocessableEntity, "valid_from: use AAAA-MM-DD", "VALIDATION")
		return
	}
	var to *time.Time
	if body.ValidTo != "" {
		t, err := time.Parse("2006-01-02", body.ValidTo)
		if err != nil {
			helpers.WriteError(w, http.StatusUnprocessableEntity, "valid_to: use AAAA-MM-DD", "VALIDATION")
			return
		}
		to = &t
	}
	rate, err := h.svc.AddRate(r.Context(), entity.RateKind(body.Kind), body.UF, body.CityCode, body.Value, from, to, body.Source)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusCreated, toRateResponse(rate))
}

// ListRates atende GET /v1/admin/tax-rates.
func (h *TaxRuleHandler) ListRates(w http.ResponseWriter, r *http.Request) {
	page := pagination.Parse(r, "valid_from")
	items, total, err := h.svc.ListRates(r.Context(), page)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	out := make([]response.Rate, 0, len(items))
	for _, it := range items {
		out = append(out, toRateResponse(it))
	}
	helpers.WriteJSON(w, http.StatusOK, pagination.NewPage(out, page, total))
}

// Simulate atende POST /v1/admin/tax/simulate.
func (h *TaxRuleHandler) Simulate(w http.ResponseWriter, r *http.Request) {
	var body request.SimulateRequest
	if err := helpers.DecodeJSON(r, &body); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido: "+err.Error(), "INVALID_BODY")
		return
	}
	date := time.Now()
	if body.Date != "" {
		d, err := time.Parse("2006-01-02", body.Date)
		if err != nil {
			helpers.WriteError(w, http.StatusUnprocessableEntity, "date: use AAAA-MM-DD", "VALIDATION")
			return
		}
		date = d
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	q := usecase.Simulation{TenantID: tenant, Date: date, UF: strings.ToUpper(body.UF), CityCode: body.CityCode, Interstate: body.Interstate}
	for _, it := range body.Items {
		q.Items = append(q.Items, usecase.SimulationItem{ProfileCode: it.Profile, Item: tax.Item{
			Origin: it.Origin, Quantity: it.Quantity, UnitPrice: it.UnitPrice, Discount: money(it.Discount),
			Freight: money(it.Freight), Insurance: money(it.Insurance), Other: money(it.Other)}})
	}
	res, err := h.svc.Simulate(r.Context(), q)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, toSimulationResponse(res))
}

// money leva os valores de entrada à escala do leiaute (2 casas). Mais casas que isso é
// recusado no item seguinte pela escala do vProd; aqui só se normaliza "5" para "5.00".
func money(v decimal.Fixed) decimal.Fixed {
	out, _ := v.Rescale(2, decimal.HalfUp)
	return out
}

// writeDomainError é o ponto único de tradução erro → status do módulo (Art. 10).
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrProfileNotFound):
		helpers.WriteError(w, http.StatusNotFound, err.Error(), "TAX_PROFILE_NOT_FOUND")
	case errors.Is(err, domain.ErrRateOverlap):
		helpers.WriteError(w, http.StatusConflict, err.Error(), "TAX_RATE_OVERLAP")
	case errors.Is(err, domain.ErrNoRate):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "TAX_RATE_MISSING")
	case errors.Is(err, domain.ErrInvalidCode), errors.Is(err, domain.ErrRequiredField),
		errors.Is(err, domain.ErrInvalidProfile), errors.Is(err, domain.ErrInvalidRate),
		errors.Is(err, domain.ErrInvalidPeriod), errors.Is(err, tax.ErrInvalidItem),
		errors.Is(err, tax.ErrUnsupportedCST), errors.Is(err, tax.ErrRegime):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "VALIDATION")
	default:
		helpers.WriteInternalError(w, r, err)
	}
}
