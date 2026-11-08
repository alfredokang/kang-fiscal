package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/service"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

// Cancel cancela uma nota autorizada.
//
// Pedir de novo é seguro: um cancelamento já registrado é devolvido como está, e um que
// ficou incerto é reenviado — se tinha chegado, a SEFAZ responde duplicidade (573).
//
// O prazo (24 horas na maioria das UFs) é a SEFAZ quem confere: as regras variam por UF e
// mudam, e recusar aqui com uma regra desatualizada impediria um cancelamento válido.
func (s *InvoiceService) Cancel(ctx context.Context, tenantID, accessKey, justification string) (*entity.Event, error) {
	inv, events, err := s.loadForEvent(ctx, tenantID, accessKey)
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		if e.Type == nfe.EventCancellation && e.Status == entity.EventRegistered {
			return e, nil
		}
		if e.Type == nfe.EventCancellation && e.Pending() {
			return s.sendEvent(ctx, inv, e)
		}
	}
	if inv.Status != entity.StatusAuthorized {
		return nil, domain.ErrNotAuthorized
	}
	party, err := s.party(inv)
	if err != nil {
		return nil, err
	}
	ev, err := nfe.NewCancellation(party, inv.Protocol, justification)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidRequest, err)
	}
	return s.createAndSend(ctx, inv, ev, 1, ev.InfEvento.DetEvento.XJust)
}

// Correct registra uma carta de correção.
//
// Cada nova CC-e SUBSTITUI as anteriores na SEFAZ: o texto precisa trazer todas as
// correções da nota, não só a nova. A CC-e não corrige valor, imposto, quantidade,
// destinatário nem data (o texto do xCondUso diz isso a quem recebe a nota).
func (s *InvoiceService) Correct(ctx context.Context, tenantID, accessKey, text string) (*entity.Event, error) {
	inv, events, err := s.loadForEvent(ctx, tenantID, accessKey)
	if err != nil {
		return nil, err
	}
	if inv.Status == entity.StatusCanceled {
		return nil, domain.ErrAlreadyCanceled
	}
	if inv.Status != entity.StatusAuthorized {
		return nil, domain.ErrNotAuthorized
	}
	seq := 1
	for _, e := range events {
		if e.Type != nfe.EventCorrection {
			continue
		}
		if e.Pending() {
			// Uma CC-e sem resposta conclusiva vai antes de qualquer nova.
			return s.sendEvent(ctx, inv, e)
		}
		if e.Status == entity.EventRegistered && e.Seq >= seq {
			seq = e.Seq + 1
		}
	}
	if seq > 20 {
		return nil, domain.ErrTooManyCorrections
	}
	party, err := s.party(inv)
	if err != nil {
		return nil, err
	}
	ev, err := nfe.NewCorrection(party, seq, text)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidRequest, err)
	}
	return s.createAndSend(ctx, inv, ev, seq, ev.InfEvento.DetEvento.XCorrecao)
}

// Events lista os eventos da nota.
func (s *InvoiceService) Events(ctx context.Context, tenantID, accessKey string) ([]*entity.Event, error) {
	if _, err := s.d.Repo.FindByAccessKey(ctx, tenantID, accessKey); err != nil {
		return nil, err
	}
	return s.d.Events.Events(ctx, tenantID, accessKey)
}

// Event lê um evento.
func (s *InvoiceService) Event(ctx context.Context, tenantID, id string) (*entity.Event, error) {
	return s.d.Events.FindEvent(ctx, tenantID, id)
}

func (s *InvoiceService) loadForEvent(ctx context.Context, tenantID, accessKey string) (*entity.Invoice, []*entity.Event, error) {
	inv, err := s.d.Repo.FindByAccessKey(ctx, tenantID, accessKey)
	if err != nil {
		return nil, nil, err
	}
	events, err := s.d.Events.Events(ctx, tenantID, accessKey)
	return inv, events, err
}

// party monta quem registra o evento a partir da própria nota: cOrgao é o cUF da chave e o
// CNPJ é o do emitente na chave — o mesmo da nota, por construção.
func (s *InvoiceService) party(inv *entity.Invoice) (nfe.EventParty, error) {
	cuf, err := strconv.Atoi(inv.AccessKey[:2])
	if err != nil {
		return nfe.EventParty{}, err
	}
	tpAmb := 2
	if inv.Environment == "production" {
		tpAmb = 1
	}
	return nfe.EventParty{OrgCode: cuf, TpAmb: tpAmb, CNPJ: inv.AccessKey[6:20], Key: inv.AccessKey,
		When: s.d.Now().In(service.TimezoneForUFCode(cuf))}, nil
}

func (s *InvoiceService) createAndSend(ctx context.Context, inv *entity.Invoice, ev *nfe.Evento, seq int, text string) (*entity.Event, error) {
	cert, err := s.d.Signers.SignerFor(ctx, inv.TenantID, inv.IssuerRef)
	if err != nil {
		return nil, err
	}
	raw, err := nfe.MarshalEvent(ev)
	if err != nil {
		return nil, err
	}
	signed, err := xmldsig.Sign(raw, "infEvento", cert)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := s.d.Now()
	e := &entity.Event{ID: id.String(), TenantID: inv.TenantID, InvoiceID: inv.ID, AccessKey: inv.AccessKey,
		Type: ev.InfEvento.TpEvento, Seq: seq, Status: entity.EventSigned, Text: text, SignedXML: signed,
		CreatedAt: now, UpdatedAt: now}
	if err := s.d.Events.CreateEvent(ctx, e); err != nil {
		return nil, err
	}
	return s.sendEventWith(ctx, inv, e, cert)
}

func (s *InvoiceService) sendEvent(ctx context.Context, inv *entity.Invoice, e *entity.Event) (*entity.Event, error) {
	cert, err := s.d.Signers.SignerFor(ctx, inv.TenantID, inv.IssuerRef)
	if err != nil {
		return nil, err
	}
	return s.sendEventWith(ctx, inv, e, cert)
}

func (s *InvoiceService) sendEventWith(ctx context.Context, inv *entity.Invoice, e *entity.Event, cert *certificate.Certificate) (*entity.Event, error) {
	issuer, _, err := s.d.Issuers.IssuerForInvoice(ctx, inv.TenantID, inv.IssuerRef)
	if err != nil {
		return nil, err
	}
	res, err := s.d.Sefaz.SendEvent(ctx, cert, s.eventTarget(ctx, inv, issuer.Address.UF), time.Now().UnixMilli(), e.SignedXML)
	now := s.d.Now()
	e.UpdatedAt = now
	switch {
	case errors.Is(err, sefaz.ErrCertificateRejected), errors.Is(err, sefaz.ErrServerUntrusted):
		e.Status, e.Motive = entity.EventNotSent, err.Error()
	case err != nil:
		e.Status, e.Motive = entity.EventUncertain, err.Error()
	case res.BatchCStat != 128:
		e.Status, e.CStat, e.Motive = entity.EventRejected, res.BatchCStat, res.BatchMotive
	case res.Registered():
		e.Status, e.CStat, e.Motive, e.Protocol = entity.EventRegistered, res.CStat, res.Motive, res.Protocol
		e.ProcXML = nfe.EventProc(e.SignedXML, res.RetEvento)
	case res.CStat == entity.EventDuplicate:
		// O envio anterior chegou: o evento já está registrado na SEFAZ. O protocolo dele
		// vem na consulta da nota; aqui fica o registro sem o procEventoNFe.
		e.Status, e.CStat, e.Motive = entity.EventRegistered, res.CStat, res.Motive
	default:
		e.Status, e.CStat, e.Motive = entity.EventRejected, res.CStat, res.Motive
	}
	if err := s.d.Events.UpdateEvent(ctx, e); err != nil {
		return nil, err
	}
	if e.Status == entity.EventRegistered && e.Type == nfe.EventCancellation && inv.Status == entity.StatusAuthorized {
		inv.Cancel(now)
		if err := s.d.Repo.Update(ctx, inv); err != nil {
			return nil, err
		}
	}
	return e, nil
}
