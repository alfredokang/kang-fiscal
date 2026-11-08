// Package usecase orquestra a emissão da NFS-e Nacional.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/decimal"
	"github.com/alfredokang/kang-fiscal/pkg/nfse"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// Service é a fachada do módulo.
type Service struct {
	repo    output.Repository
	issuers output.IssuerReader
	signers output.SignerProvider
	numbers output.NumberReserver
	gw      output.Gateway
	now     func() time.Time
}

// NewService monta o serviço.
func NewService(repo output.Repository, issuers output.IssuerReader, signers output.SignerProvider, numbers output.NumberReserver, gw output.Gateway) *Service {
	return &Service{repo: repo, issuers: issuers, signers: signers, numbers: numbers, gw: gw, now: func() time.Time { return time.Now().UTC() }}
}

// Recipient é o tomador.
type Recipient struct {
	CNPJ, CPF, Name, Email, Phone        string
	Street, Number, District, Complement string
	CityCode, ZipCode                    string
}

// Draft é o pedido de emissão.
type Draft struct {
	Series                int
	Competence            time.Time
	NationalCode          string // cTribNac
	MunicipalCode, NBS    string
	Description           string
	ServiceCity           string // cLocPrestacao; vazio = o município do prestador
	Amount                decimal.Fixed
	UnconditionalDiscount decimal.Fixed
	ISSTaxation           int // tribISSQN, padrão 1
	ISSRetention          int // tpRetISSQN, padrão 1
	ISSRate               *decimal.Fixed
	PisCofinsCST          string
	SimplesTaxPercent     *decimal.Fixed // pTotTribSN, obrigatório no Simples (ME/EPP)
	Recipient             *Recipient
	IBSCBS                *nfse.IBSCBS
}

// IssueCommand é o pedido com a idempotência.
type IssueCommand struct {
	TenantID, IssuerRef, IdempotencyKey, RequestHash string
	Draft                                            Draft
}

// Issue emite a NFS-e, na ordem da Constituição, Art. 6.1: idempotência, número, DPS
// assinada e gravada — e só então o envio. Resposta incerta vira consulta da DPS pelo Id.
func (s *Service) Issue(ctx context.Context, cmd IssueCommand) (*entity.ServiceInvoice, error) {
	idem := strings.TrimSpace(cmd.IdempotencyKey)
	if idem == "" || len(idem) > 80 {
		return nil, domain.ErrIdempotencyKey
	}
	if existing, err := s.repo.FindByIdempotencyKey(ctx, cmd.TenantID, idem); err == nil {
		if existing.RequestHash != cmd.RequestHash {
			return nil, domain.ErrIdempotencyConflict
		}
		return s.resume(ctx, existing)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	issuer, err := s.issuers.IssuerForNFSe(ctx, cmd.TenantID, cmd.IssuerRef)
	if err != nil {
		return nil, err
	}
	d := cmd.Draft
	if err := validate(d, issuer); err != nil {
		return nil, err
	}
	cert, err := s.signers.SignerForNFSe(ctx, cmd.TenantID, cmd.IssuerRef)
	if err != nil {
		return nil, err
	}
	if d.Series == 0 {
		d.Series = 1
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	ref, number, docID, err := s.numbers.ReserveDPS(ctx, cmd.TenantID, cmd.IssuerRef, issuer.Environment, d.Series, id.String(), idem)
	if err != nil {
		return nil, err
	}
	dps, err := build(d, issuer, number, s.now())
	if err != nil {
		return nil, err
	}
	raw, err := nfse.Marshal(dps)
	if err != nil {
		return nil, err
	}
	signed, err := xmldsig.Sign(raw, "infDPS", cert)
	if err != nil {
		return nil, err
	}
	now := s.now()
	inv := &entity.ServiceInvoice{ID: docID, TenantID: cmd.TenantID, IssuerRef: cmd.IssuerRef, IdempotencyKey: idem,
		RequestHash: cmd.RequestHash, Series: d.Series, Number: number, ReservationRef: ref, Environment: issuer.Environment,
		DPSID: dps.InfDPS.ID, Status: entity.StatusSigned, Total: d.Amount.String(), SignedXML: signed, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Create(ctx, inv); err != nil {
		// A mesma venda chegou duas vezes ao mesmo tempo: a mesma reserva e o mesmo Id de DPS,
		// e só uma gravou. A outra devolve a gravada, sem reenviar.
		existing, ferr := s.repo.FindByIdempotencyKey(ctx, cmd.TenantID, idem)
		if ferr != nil {
			return nil, err
		}
		if existing.RequestHash != cmd.RequestHash {
			return nil, domain.ErrIdempotencyConflict
		}
		return existing, nil
	}
	return s.send(ctx, inv, cert, true)
}

func env(e string) int {
	if e == "production" {
		return 1
	}
	return 2
}

func (s *Service) send(ctx context.Context, inv *entity.ServiceInvoice, cert *certificate.Certificate, allowRetry bool) (*entity.ServiceInvoice, error) {
	res, err := s.gw.Issue(ctx, cert, env(inv.Environment), inv.SignedXML)
	switch {
	case errors.Is(err, nfse.ErrCertificateRejected):
		inv.Settle(entity.StatusNotSent, "", err.Error(), s.now())
		if uerr := s.repo.Update(ctx, inv); uerr != nil {
			return nil, uerr
		}
		return inv, fmt.Errorf("%w: %v", domain.ErrNotSent, err)
	case err != nil:
		inv.Settle(entity.StatusUncertain, "", err.Error(), s.now())
		if uerr := s.repo.Update(ctx, inv); uerr != nil {
			return nil, uerr
		}
		if allowRetry {
			return s.reconcile(ctx, inv, cert, false)
		}
		return inv, nil
	case res.Authorized():
		inv.Authorize(res.AccessKey, res.Number, res.NFSeXML, s.now())
	default:
		code, msg := "", ""
		if len(res.Errors) > 0 {
			e := res.Errors[0]
			code, msg = e.Code, strings.TrimSpace(e.Description+" "+e.Complement)
		}
		inv.Settle(entity.StatusRejected, code, msg, s.now())
	}
	if err := s.repo.Update(ctx, inv); err != nil {
		return nil, err
	}
	// A DPS não tem inutilização: autorizada ou recusada, a reserva do número se fecha.
	_ = s.numbers.CloseDPS(ctx, inv.TenantID, inv.ReservationRef)
	return inv, nil
}

// reconcile consulta a DPS pelo Id: se já gerou NFS-e, baixa e registra; se não existe, e
// só então, reenvia a MESMA DPS (uma vez).
func (s *Service) reconcile(ctx context.Context, inv *entity.ServiceInvoice, cert *certificate.Certificate, allowResend bool) (*entity.ServiceInvoice, error) {
	key, found, err := s.gw.DPSKey(ctx, cert, env(inv.Environment), inv.DPSID)
	if err != nil {
		return inv, nil // continua incerta
	}
	if found {
		doc, ok, err := s.gw.Get(ctx, cert, env(inv.Environment), key)
		if err != nil || !ok {
			return inv, nil
		}
		inv.Authorize(key, number(doc), doc, s.now())
		if err := s.repo.Update(ctx, inv); err != nil {
			return nil, err
		}
		_ = s.numbers.CloseDPS(ctx, inv.TenantID, inv.ReservationRef)
		return inv, nil
	}
	if allowResend {
		return s.send(ctx, inv, cert, false)
	}
	return inv, nil
}

func (s *Service) resume(ctx context.Context, inv *entity.ServiceInvoice) (*entity.ServiceInvoice, error) {
	if inv.Status != entity.StatusUncertain && inv.Status != entity.StatusSigned {
		return inv, nil
	}
	cert, err := s.signers.SignerForNFSe(ctx, inv.TenantID, inv.IssuerRef)
	if err != nil {
		return nil, err
	}
	return s.reconcile(ctx, inv, cert, true)
}

// Get lê uma NFS-e pela chave.
func (s *Service) Get(ctx context.Context, tenantID, key string) (*entity.ServiceInvoice, error) {
	return s.repo.FindByKey(ctx, tenantID, key)
}

// List lista as NFS-e do tenant.
func (s *Service) List(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.ServiceInvoice, int64, error) {
	return s.repo.List(ctx, tenantID, page)
}

func number(doc []byte) string {
	str := string(doc)
	i := strings.Index(str, "<nNFSe>")
	j := strings.Index(str, "</nNFSe>")
	if i < 0 || j < i {
		return ""
	}
	return str[i+7 : j]
}

func validate(d Draft, issuer output.Issuer) error {
	if len(d.NationalCode) != 6 || strings.Trim(d.NationalCode, "0123456789") != "" {
		return fmt.Errorf("%w: código de tributação nacional (cTribNac) tem 6 dígitos", domain.ErrInvalidRequest)
	}
	if strings.TrimSpace(d.Description) == "" || d.Amount.IsZero() || d.Amount.IsNegative() {
		return fmt.Errorf("%w: descrição do serviço e valor são obrigatórios", domain.ErrInvalidRequest)
	}
	if issuer.CityCode == "" {
		return fmt.Errorf("%w: código IBGE do município", domain.ErrIssuerIncomplete)
	}
	if opSimpNac(issuer.CRT) == 3 && d.SimplesTaxPercent == nil {
		return fmt.Errorf("%w: no Simples Nacional (ME/EPP) informe simples_tax_percent", domain.ErrInvalidRequest)
	}
	if d.ISSRate != nil && d.ISSRate.Cmp(decimal.MustNew(10, 0)) >= 0 {
		return fmt.Errorf("%w: alíquota do ISS até 9,99%%", domain.ErrInvalidRequest)
	}
	return nil
}

// opSimpNac é a situação no Simples pelo CRT do emitente: 1 não optante, 2 MEI, 3 ME/EPP.
func opSimpNac(crt int) int {
	switch crt {
	case 3:
		return 1
	case 4:
		return 2
	}
	return 3
}

func money(v decimal.Fixed) decimal.Fixed {
	out, _ := v.Rescale(2, decimal.HalfUp)
	return out
}

func build(d Draft, issuer output.Issuer, n int, now time.Time) (*nfse.DPS, error) {
	id, err := nfse.ID(issuer.CityCode, issuer.CNPJ, d.Series, n)
	if err != nil {
		return nil, err
	}
	tz := time.FixedZone("-03", -3*3600)
	comp := d.Competence
	if comp.IsZero() {
		comp = now
	}
	loc := d.ServiceCity
	if loc == "" {
		loc = issuer.CityCode
	}
	op := opSimpNac(issuer.CRT)
	reg := nfse.RegTrib{OpSimpNac: op, RegEspTrib: 0}
	if op == 3 {
		ap := 1
		reg.RegApTribSN = &ap
	}
	tpAmb := env(issuer.Environment)
	dps := &nfse.DPS{InfDPS: nfse.InfDPS{
		ID: id, TpAmb: tpAmb, DhEmi: nfse.Timestamp(now.In(tz)), VerAplic: "kang-fiscal",
		Serie: strconv.Itoa(d.Series), NDPS: strconv.Itoa(n), DCompet: comp.In(tz).Format("2006-01-02"),
		TpEmit: 1, FinNFSe: 0, CLocEmi: issuer.CityCode,
		Prest: nfse.Prest{CNPJ: issuer.CNPJ, IM: issuer.MunicipalRegistration, RegTrib: reg},
		Serv: nfse.Serv{LocPrest: nfse.LocPrest{CLocPrestacao: loc}, CServ: nfse.CServ{CTribNac: d.NationalCode,
			CTribMun: d.MunicipalCode, CNBS: d.NBS, XDescServ: strings.Join(strings.Fields(d.Description), " ")}},
		Valores: nfse.Valores{VServPrest: nfse.VServPrest{VServ: money(d.Amount)},
			Trib: nfse.Trib{TribMun: nfse.TribMun{TribISSQN: orDefault(d.ISSTaxation, 1), TpRetISSQN: orDefault(d.ISSRetention, 1)}}},
		IBSCBS: d.IBSCBS,
	}}
	v := &dps.InfDPS.Valores
	if !d.UnconditionalDiscount.IsZero() {
		disc := money(d.UnconditionalDiscount)
		v.VDescCondIncond = &nfse.VDescCondIncond{VDescIncond: &disc}
	}
	if d.ISSRate != nil {
		r, _ := d.ISSRate.Rescale(2, decimal.HalfUp)
		v.Trib.TribMun.PAliq = &r
	}
	if d.PisCofinsCST != "" {
		v.Trib.TribFed = &nfse.TribFed{PisCofins: &nfse.PisCofins{CST: d.PisCofinsCST}}
	}
	if op == 3 {
		p, _ := d.SimplesTaxPercent.Rescale(2, decimal.HalfUp)
		v.Trib.TotTrib = nfse.TotTrib{PTotTribSN: &p}
	} else {
		zero := 0
		v.Trib.TotTrib = nfse.TotTrib{IndTotTrib: &zero}
	}
	if r := d.Recipient; r != nil && (r.CNPJ != "" || r.CPF != "") {
		p := &nfse.Pessoa{CNPJ: r.CNPJ, CPF: r.CPF, XNome: strings.Join(strings.Fields(r.Name), " "), Email: r.Email, Fone: r.Phone}
		if r.CityCode != "" && r.ZipCode != "" {
			p.End = &nfse.Endereco{EndNac: nfse.EndNac{CMun: r.CityCode, CEP: r.ZipCode}, XLgr: r.Street, Nro: r.Number,
				XCpl: r.Complement, XBairro: r.District}
		}
		dps.InfDPS.Toma = p
	}
	return dps, nil
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
