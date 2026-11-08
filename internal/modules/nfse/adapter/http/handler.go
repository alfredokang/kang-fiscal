// Package http é a entrada HTTP da NFS-e Nacional.
package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfse"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// Handler atende as rotas.
type Handler struct{ svc *usecase.Service }

// NewHandler monta o handler.
func NewHandler(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

type issueRequest struct {
	IssuerID   string `json:"issuer_id"`
	Series     int    `json:"series"`
	Competence string `json:"competence"` // AAAA-MM-DD; vazio = hoje
	Service    struct {
		NationalCode  string `json:"national_code"` // cTribNac, 6 dígitos (LC 116)
		MunicipalCode string `json:"municipal_code"`
		NBS           string `json:"nbs"`
		Description   string `json:"description"`
		CityCode      string `json:"city_code"` // onde foi prestado; vazio = município do prestador
	} `json:"service"`
	Amount                decimal.Fixed `json:"amount"`
	UnconditionalDiscount decimal.Fixed `json:"unconditional_discount"`
	ISS                   struct {
		Taxation  int            `json:"taxation"`  // 1 tributável, 2 imunidade, 3 exportação, 4 não incidência
		Retention int            `json:"retention"` // 1 não retido, 2 pelo tomador, 3 pelo intermediário
		Rate      *decimal.Fixed `json:"rate"`
	} `json:"iss"`
	PisCofinsCST      string         `json:"pis_cofins_cst"`
	SimplesTaxPercent *decimal.Fixed `json:"simples_tax_percent"`
	Recipient         *struct {
		CNPJ       string `json:"cnpj"`
		CPF        string `json:"cpf"`
		Name       string `json:"name"`
		Email      string `json:"email"`
		Phone      string `json:"phone"`
		Street     string `json:"street"`
		Number     string `json:"number"`
		District   string `json:"district"`
		Complement string `json:"complement"`
		CityCode   string `json:"city_code"`
		ZipCode    string `json:"zip_code"`
	} `json:"recipient"`
	IBSCBS *struct {
		CST           string `json:"cst"`
		ClassTrib     string `json:"class_trib"`
		FinalConsumer bool   `json:"final_consumer"`
		OperationCode string `json:"operation_code"`
	} `json:"ibs_cbs"`
}

type invoiceResponse struct {
	ID          string    `json:"id"`
	DPSID       string    `json:"dps_id"`
	AccessKey   string    `json:"access_key,omitempty"`
	NFSeNumber  string    `json:"nfse_number,omitempty"`
	Series      int       `json:"series"`
	Number      int       `json:"dps_number"`
	Environment string    `json:"environment"`
	Status      string    `json:"status"`
	ErrorCode   string    `json:"error_code,omitempty"`
	Message     string    `json:"message,omitempty"`
	Total       string    `json:"total"`
	XMLHash     string    `json:"xml_sha256,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func toResponse(s *entity.ServiceInvoice) invoiceResponse {
	return invoiceResponse{ID: s.ID, DPSID: s.DPSID, AccessKey: s.AccessKey, NFSeNumber: s.NFSeNumber, Series: s.Series,
		Number: s.Number, Environment: s.Environment, Status: string(s.Status), ErrorCode: s.ErrorCode, Message: s.Message,
		Total: s.Total, XMLHash: s.XMLHash, CreatedAt: s.CreatedAt}
}

// Issue atende POST /v1/nfse.
func (h *Handler) Issue(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, helpers.MaxBodyBytes))
	if err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo ilegível", "INVALID_BODY")
		return
	}
	var b issueRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "corpo inválido: "+err.Error(), "INVALID_BODY")
		return
	}
	d := usecase.Draft{Series: b.Series, NationalCode: digits(b.Service.NationalCode), MunicipalCode: b.Service.MunicipalCode,
		NBS: b.Service.NBS, Description: b.Service.Description, ServiceCity: b.Service.CityCode, Amount: b.Amount,
		UnconditionalDiscount: b.UnconditionalDiscount, ISSTaxation: b.ISS.Taxation, ISSRetention: b.ISS.Retention,
		ISSRate: b.ISS.Rate, PisCofinsCST: b.PisCofinsCST, SimplesTaxPercent: b.SimplesTaxPercent}
	if b.Competence != "" {
		c, err := time.Parse("2006-01-02", b.Competence)
		if err != nil {
			helpers.WriteError(w, http.StatusUnprocessableEntity, "competence: AAAA-MM-DD", "VALIDATION")
			return
		}
		d.Competence = c
	}
	if rc := b.Recipient; rc != nil {
		d.Recipient = &usecase.Recipient{CNPJ: digits(rc.CNPJ), CPF: digits(rc.CPF), Name: rc.Name, Email: rc.Email,
			Phone: digits(rc.Phone), Street: rc.Street, Number: rc.Number, District: rc.District, Complement: rc.Complement,
			CityCode: rc.CityCode, ZipCode: digits(rc.ZipCode)}
	}
	if g := b.IBSCBS; g != nil {
		x := &nfse.IBSCBS{CIndOp: g.OperationCode, IndDest: 0,
			Valores: nfse.IBSCBSValores{Trib: nfse.IBSCBSTrib{CST: g.CST, CClassTrib: g.ClassTrib}}}
		if g.FinalConsumer {
			one := 1
			x.IndFinal = &one
		}
		d.IBSCBS = x
	}
	tenant, _ := middleware.TenantIDFrom(r.Context())
	inv, err := h.svc.Issue(r.Context(), usecase.IssueCommand{TenantID: tenant, IssuerRef: b.IssuerID,
		IdempotencyKey: r.Header.Get("Idempotency-Key"), RequestHash: hash(raw), Draft: d})
	if err != nil && inv == nil {
		writeError(w, r, err)
		return
	}
	status := http.StatusCreated
	switch inv.Status {
	case entity.StatusRejected:
		status = http.StatusUnprocessableEntity
	case entity.StatusUncertain, entity.StatusSigned:
		status = http.StatusAccepted
	case entity.StatusNotSent:
		status = http.StatusBadGateway
	}
	helpers.WriteJSON(w, status, toResponse(inv))
}

// Get atende GET /v1/nfse/{key} (chave de acesso ou Id da DPS).
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	inv, err := h.svc.Get(r.Context(), tenant, chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, toResponse(inv))
}

// XML atende GET /v1/nfse/{key}/xml: a NFS-e, ou a DPS assinada enquanto não houver nota.
func (h *Handler) XML(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	inv, err := h.svc.Get(r.Context(), tenant, chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	doc, name := inv.SignedXML, inv.DPSID+".xml"
	if len(inv.NFSeXML) > 0 {
		doc, name = inv.NFSeXML, "NFSe"+inv.AccessKey+".xml"
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write(doc)
}

// List atende GET /v1/nfse.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.TenantIDFrom(r.Context())
	page := pagination.Parse(r, "created_at")
	items, total, err := h.svc.List(r.Context(), tenant, page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]invoiceResponse, 0, len(items))
	for _, s := range items {
		out = append(out, toResponse(s))
	}
	helpers.WriteJSON(w, http.StatusOK, pagination.NewPage(out, page, total))
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrIssuerNotFound):
		helpers.WriteError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
	case errors.Is(err, domain.ErrIdempotencyConflict):
		helpers.WriteError(w, http.StatusConflict, err.Error(), "IDEMPOTENCY_CONFLICT")
	case errors.Is(err, domain.ErrNoCertificate):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "NO_CERTIFICATE")
	case errors.Is(err, domain.ErrIdempotencyKey), errors.Is(err, domain.ErrInvalidRequest),
		errors.Is(err, domain.ErrIssuerIncomplete), errors.Is(err, nfse.ErrInvalid):
		helpers.WriteError(w, http.StatusUnprocessableEntity, err.Error(), "VALIDATION")
	default:
		helpers.WriteInternalError(w, r, err)
	}
}

func digits(s string) string {
	var b strings.Builder
	for _, c := range strings.ToUpper(s) {
		if c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// RegisterRoutes pendura as rotas.
func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/nfse", h.Issue)
	r.Get("/nfse", h.List)
	r.Get("/nfse/{key}", h.Get)
	r.Get("/nfse/{key}/xml", h.XML)
}
