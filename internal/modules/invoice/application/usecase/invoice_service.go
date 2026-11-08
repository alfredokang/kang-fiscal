// Package usecase orquestra a emissão.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/service"
	"github.com/alfredokang/kang-fiscal/pkg/accesskey"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// Deps são as portas da emissão.
type Deps struct {
	Repo        output.InvoiceRepository
	Events      output.EventRepository
	Voids       output.VoidRepository
	Contingency output.ContingencyRepository
	Issuers     output.IssuerReader
	Signers     output.SignerProvider
	Profiles    output.TaxProfiles
	Rates       output.TaxRates
	Numbers     output.NumberReserver
	Sefaz       output.SefazGateway
	RespTec     *nfe.InfRespTec
	Now         func() time.Time
}

// InvoiceService é a fachada do módulo.
type InvoiceService struct{ d Deps }

// NewInvoiceService monta o serviço.
func NewInvoiceService(d Deps) *InvoiceService {
	if d.Now == nil {
		d.Now = func() time.Time { return time.Now().UTC() }
	}
	return &InvoiceService{d: d}
}

// IssueCommand é o pedido de emissão.
type IssueCommand struct {
	TenantID       string
	IssuerRef      string
	IdempotencyKey string
	// RequestHash é a impressão digital do corpo, calculada pelo adapter.
	RequestHash string
	Draft       service.Draft
}

// Issue emite a nota.
//
// A ordem é a da Constituição, Art. 6.1: idempotência, número, tributos, XML assinado e
// chave GRAVADOS — e só então o envio. Qualquer falha depois do envio deixa a nota com
// estado e chave conhecidos, e nunca leva a reenviar sem consultar.
func (s *InvoiceService) Issue(ctx context.Context, cmd IssueCommand) (*entity.Invoice, error) {
	key := strings.TrimSpace(cmd.IdempotencyKey)
	if key == "" || len(key) > 80 {
		return nil, domain.ErrIdempotencyKey
	}
	if existing, err := s.d.Repo.FindByIdempotencyKey(ctx, cmd.TenantID, key); err == nil {
		if existing.RequestHash != cmd.RequestHash {
			return nil, domain.ErrIdempotencyConflict
		}
		return s.resume(ctx, existing)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	issuer, missing, err := s.d.Issuers.IssuerForInvoice(ctx, cmd.TenantID, cmd.IssuerRef)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: falta %s", domain.ErrIssuerIncomplete, strings.Join(missing, ", "))
	}
	draft := cmd.Draft
	draft.RespTec = s.d.RespTec
	if draft.Issued.IsZero() {
		draft.Issued = s.d.Now()
	}
	if draft.Series == 0 {
		draft.Series = 1
	}
	if err := draft.Validate(issuer); err != nil {
		return nil, err
	}
	// O certificado é conferido antes de reservar número: sem ele a nota não sai, e um
	// número reservado à toa vira buraco a inutilizar.
	cert, err := s.d.Signers.SignerFor(ctx, cmd.TenantID, cmd.IssuerRef)
	if err != nil {
		return nil, err
	}
	if draft.Model == 55 {
		if c, err := s.d.Contingency.Get(ctx, strings.ToUpper(issuer.Address.UF), 55, issuer.Environment); err == nil && c.Active {
			draft.Contingency = &c
		}
	}
	taxes, err := s.computeTaxes(ctx, cmd.TenantID, draft, issuer)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	resRef, number, docID, err := s.d.Numbers.ReserveNumber(ctx, output.NumberKey{TenantID: cmd.TenantID, IssuerRef: cmd.IssuerRef,
		Environment: issuer.Environment, Model: draft.Model, Series: draft.Series}, id.String(), key)
	if err != nil {
		return nil, err
	}
	code, err := accesskey.NewCode(number)
	if err != nil {
		return nil, err
	}
	doc, akey, err := service.Build(draft, issuer, number, code, taxes)
	if err != nil {
		return nil, err
	}
	raw, err := nfe.Marshal(doc)
	if err != nil {
		return nil, err
	}
	signed, err := xmldsig.Sign(raw, "infNFe", cert)
	if err != nil {
		return nil, err
	}

	now := s.d.Now()
	inv := &entity.Invoice{
		ID: docID, TenantID: cmd.TenantID, IssuerRef: cmd.IssuerRef, IdempotencyKey: key,
		RequestHash: cmd.RequestHash, Model: draft.Model, Series: draft.Series, Number: number,
		ReservationRef: resRef, Environment: issuer.Environment, AccessKey: akey.String(),
		Status: entity.StatusSigned, TotalAmount: doc.InfNFe.Total.ICMSTot.VNF.String(), SignedXML: signed,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.d.Repo.Create(ctx, inv); err != nil {
		return s.lostRace(ctx, cmd, err)
	}
	return s.send(ctx, inv, cert, issuer.Address.UF, true)
}

// lostRace trata a gravação recusada. A causa comum é a mesma venda chegando duas vezes ao
// mesmo tempo: as duas reservaram o mesmo número e só uma gravou. A que perdeu devolve a
// nota da outra como está — sem reenviar: quem ganhou está enviando agora.
func (s *InvoiceService) lostRace(ctx context.Context, cmd IssueCommand, cause error) (*entity.Invoice, error) {
	existing, err := s.d.Repo.FindByIdempotencyKey(ctx, cmd.TenantID, strings.TrimSpace(cmd.IdempotencyKey))
	if err != nil {
		return nil, cause
	}
	if existing.RequestHash != cmd.RequestHash {
		return nil, domain.ErrIdempotencyConflict
	}
	return existing, nil
}

func (s *InvoiceService) computeTaxes(ctx context.Context, tenantID string, d service.Draft, issuer service.IssuerSnapshot) ([]tax.Result, error) {
	rates, err := s.d.Rates.TaxRatesAt(ctx, d.Issued, issuer.Address.UF, issuer.Address.CityCode)
	if err != nil {
		return nil, err
	}
	profiles := map[string]tax.Profile{}
	out := make([]tax.Result, len(d.Lines))
	tctx := tax.Context{Simples: issuer.CRT != 3, Interstate: service.Interstate(d, issuer)}
	for i, l := range d.Lines {
		p, ok := profiles[l.Profile]
		if !ok {
			if p, err = s.d.Profiles.TaxProfile(ctx, tenantID, l.Profile); err != nil {
				return nil, fmt.Errorf("item %d: %w", i+1, err)
			}
			profiles[l.Profile] = p
		}
		res, err := tax.Compute(tax.Item{Origin: l.Origin, Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Discount: l.Discount, Freight: l.Freight, Insurance: l.Insurance, Other: l.Other}, p, rates, tctx)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i+1, err)
		}
		out[i] = res
	}
	return out, nil
}

func target(inv *entity.Invoice, uf string) sefaz.Target {
	env := sefaz.Homologation
	if inv.Environment == "production" {
		env = sefaz.Production
	}
	return sefaz.Target{Model: sefaz.Model(inv.Model), UF: uf, Environment: env}
}

// send envia e registra o resultado. allowRetry limita a um o ciclo consulta → reenvio: se
// o reenvio também ficar incerto, a nota fica incerta e espera a próxima consulta.
func (s *InvoiceService) send(ctx context.Context, inv *entity.Invoice, cert *certificate.Certificate, uf string, allowRetry bool) (*entity.Invoice, error) {
	res, err := s.d.Sefaz.Authorize(ctx, cert, emissionTarget(inv, uf), time.Now().UnixMilli(), inv.SignedXML)
	switch {
	case errors.Is(err, sefaz.ErrCertificateRejected), errors.Is(err, sefaz.ErrServerUntrusted):
		inv.Settle(entity.StatusNotSent, 0, err.Error(), s.d.Now())
		if uerr := s.d.Repo.Update(ctx, inv); uerr != nil {
			return nil, uerr
		}
		return inv, fmt.Errorf("%w: %v", domain.ErrNotSent, err)
	case err != nil:
		// Incerto: pode ter sido autorizada. Consulta pela chave, nunca reenvia às cegas.
		inv.Settle(entity.StatusUncertain, 0, err.Error(), s.d.Now())
		if uerr := s.d.Repo.Update(ctx, inv); uerr != nil {
			return nil, uerr
		}
		if onlineNFCe(inv) && inv.SupersededBy == "" {
			// A venda não espera a SEFAZ: sai agora uma NFC-e offline, e esta fica
			// substituída (nunca reenviada — pode ter sido autorizada).
			return s.goOffline(ctx, inv, cert, uf)
		}
		if normalEmission(inv) {
			// A nota incerta NÃO vai para a SVC: ela pode ter sido autorizada pela UF, e a
			// SVC daria outra chave à mesma venda. Só as próximas notas saem por lá.
			s.recordFailure(ctx, inv, uf, err)
		}
		if !allowRetry {
			return inv, nil
		}
		return s.reconcile(ctx, inv, cert, uf, false)
	}
	if normalEmission(inv) {
		_ = s.d.Contingency.ResetFailures(ctx, uf, inv.Model, inv.Environment)
	}
	// SEFAZ parada (108/109): o lote foi recusado, a nota com certeza não foi autorizada.
	// Liga a SVC e reemite com o mesmo número — a venda não para.
	if down := entity.IsServiceDown(res.BatchCStat) || (res.Protocol != nil && entity.IsServiceDown(res.Protocol.CStat)); down && normalEmission(inv) {
		cStat, motive := res.BatchCStat, res.BatchMotive
		if res.Protocol != nil {
			cStat, motive = res.Protocol.CStat, res.Protocol.Motive
		}
		if err := s.activate(ctx, uf, inv.Model, inv.Environment, fmt.Sprintf("SEFAZ %s parada: %d %s", uf, cStat, motive)); err == nil {
			return s.reissueInContingency(ctx, inv, cert, uf)
		}
	}
	// Duplicidade (204) é a SEFAZ dizendo que esta chave já foi recebida — um envio anterior
	// chegou lá. Não é rejeição: consulta-se o protocolo que já existe.
	if res.Protocol != nil && res.Protocol.CStat == 204 {
		return s.reconcile(ctx, inv, cert, uf, false)
	}
	if res.BatchCStat != 104 || res.Protocol == nil {
		inv.Settle(entity.StatusRejected, res.BatchCStat, res.BatchMotive, s.d.Now())
		return inv, s.d.Repo.Update(ctx, inv)
	}
	return inv, s.apply(ctx, inv, res.Protocol)
}

// apply registra o protocolo de uma nota.
func (s *InvoiceService) apply(ctx context.Context, inv *entity.Invoice, p *sefaz.Protocol) error {
	now := s.d.Now()
	switch {
	case entity.IsAuthorization(p.CStat):
		inv.Authorize(p.CStat, p.Motive, p.Number, nfe.Proc(inv.SignedXML, p.Raw), now)
	case entity.IsDenial(p.CStat):
		inv.Settle(entity.StatusDenied, p.CStat, p.Motive, now)
	default:
		inv.Settle(entity.StatusRejected, p.CStat, p.Motive, now)
	}
	if err := s.d.Repo.Update(ctx, inv); err != nil {
		return err
	}
	if inv.Status.Final() {
		// Autorizada ou denegada, o número foi consumido na SEFAZ.
		return s.d.Numbers.MarkNumberUsed(ctx, inv.TenantID, inv.ReservationRef)
	}
	return nil
}

// reconcile consulta a nota pela chave e decide o estado.
//
// 100/150 → autorizada; denegação → denegada; 217 (a SEFAZ não conhece a chave) → só agora
// é seguro reenviar o MESMO XML assinado. Sem resposta, fica incerta e a próxima chamada
// com a mesma Idempotency-Key (ou o GET com sync) tenta de novo.
func (s *InvoiceService) reconcile(ctx context.Context, inv *entity.Invoice, cert *certificate.Certificate, uf string, allowResend bool) (*entity.Invoice, error) {
	q, err := s.d.Sefaz.Query(ctx, cert, emissionTarget(inv, uf), inv.AccessKey)
	if err != nil {
		return inv, nil // continua incerta; nada a fazer sem resposta
	}
	if q.Protocol != nil && (entity.IsAuthorization(q.Protocol.CStat) || entity.IsDenial(q.Protocol.CStat)) {
		return inv, s.apply(ctx, inv, q.Protocol)
	}
	if q.CStat == 217 && allowResend {
		return s.send(ctx, inv, cert, uf, false)
	}
	return inv, nil
}

// resume continua uma nota já existente para a mesma Idempotency-Key.
func (s *InvoiceService) resume(ctx context.Context, inv *entity.Invoice) (*entity.Invoice, error) {
	if inv.SupersededBy != "" {
		// A venda foi coberta pela NFC-e offline: é ela que responde. O destino desta é do
		// worker (TransmitOffline), nunca de um reenvio.
		return s.d.Repo.FindByID(ctx, inv.TenantID, inv.SupersededBy)
	}
	if inv.Status != entity.StatusUncertain && inv.Status != entity.StatusSigned {
		return inv, nil
	}
	issuer, _, err := s.d.Issuers.IssuerForInvoice(ctx, inv.TenantID, inv.IssuerRef)
	if err != nil {
		return nil, err
	}
	cert, err := s.d.Signers.SignerFor(ctx, inv.TenantID, inv.IssuerRef)
	if err != nil {
		return nil, err
	}
	return s.reconcile(ctx, inv, cert, issuer.Address.UF, true)
}

// Get lê uma nota pela chave. Com sync, uma nota incerta é consultada na SEFAZ antes.
func (s *InvoiceService) Get(ctx context.Context, tenantID, accessKey string, sync bool) (*entity.Invoice, error) {
	inv, err := s.d.Repo.FindByAccessKey(ctx, tenantID, accessKey)
	if err != nil || !sync {
		return inv, err
	}
	return s.resume(ctx, inv)
}

// List lista as notas do tenant, as mais recentes primeiro.
func (s *InvoiceService) List(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.Invoice, int64, error) {
	return s.d.Repo.List(ctx, tenantID, page)
}
