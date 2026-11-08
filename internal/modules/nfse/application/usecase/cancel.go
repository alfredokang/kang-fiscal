package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfse"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

// CancelCommand é o pedido de cancelamento.
type CancelCommand struct {
	TenantID, Key string
	Reason        int
	Justification string
	// Analysis pede o cancelamento por análise fiscal (e101103), o único aceito onde o
	// município não deixa cancelar direto.
	Analysis bool
}

// Cancel pede o cancelamento da NFS-e. Repetir é seguro: o evento registrado é devolvido
// como está, e o que ficou sem resposta é conferido na Sefin antes de qualquer reenvio.
func (s *Service) Cancel(ctx context.Context, cmd CancelCommand) (*entity.Event, error) {
	inv, err := s.repo.FindByKey(ctx, cmd.TenantID, cmd.Key)
	if err != nil {
		return nil, err
	}
	kind := entity.KindCancel
	if cmd.Analysis {
		kind = entity.KindAnalysis
	}
	ev, err := s.repo.FindEvent(ctx, cmd.TenantID, inv.ID, kind)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if ev != nil && ev.Status == entity.EventRegistered {
		return ev, nil
	}
	if inv.Status != entity.StatusAuthorized {
		return nil, domain.ErrNotAuthorized
	}
	cert, err := s.signers.SignerForNFSe(ctx, inv.TenantID, inv.IssuerRef)
	if err != nil {
		return nil, err
	}
	if ev != nil && (ev.Status == entity.EventUncertain || ev.Status == entity.EventSigned) {
		return s.reconcileEvent(ctx, inv, ev, cert)
	}
	issuer, err := s.issuers.IssuerForNFSe(ctx, inv.TenantID, inv.IssuerRef)
	if err != nil {
		return nil, err
	}
	p, err := nfse.NewCancellation(nfse.Cancellation{AccessKey: inv.AccessKey, Author: issuer.CNPJ, Reason: cmd.Reason,
		Justification: cmd.Justification, Environment: env(inv.Environment), At: s.localNow(issuer.UF), Analysis: cmd.Analysis})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidEvent, strings.TrimPrefix(err.Error(), nfse.ErrInvalid.Error()+": "))
	}
	raw, err := nfse.MarshalEvent(p)
	if err != nil {
		return nil, err
	}
	signed, err := xmldsig.Sign(raw, "infPedReg", cert)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if ev == nil {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		ev = &entity.Event{ID: id.String(), TenantID: inv.TenantID, DocumentID: inv.ID, Kind: kind, CreatedAt: now}
		ev.Reason, ev.Justification, ev.SignedXML, ev.Status, ev.UpdatedAt = cmd.Reason, p.InfPedReg.Justification(), signed, entity.EventSigned, now
		if err := s.repo.CreateEvent(ctx, ev); err != nil {
			return nil, err
		}
	} else {
		// Tentativa anterior recusada ou não processada: refaz na mesma linha.
		ev.Reason, ev.Justification, ev.SignedXML, ev.EventXML = cmd.Reason, p.InfPedReg.Justification(), signed, nil
		ev.Settle(entity.EventSigned, "", "", now)
		if err := s.repo.UpdateEvent(ctx, ev); err != nil {
			return nil, err
		}
	}
	return s.sendEvent(ctx, inv, ev, cert)
}

// localNow é o horário do evento no fuso do prestador; a Sefin recusa horário no futuro.
func (s *Service) localNow(uf string) time.Time {
	now := s.now()
	if loc, err := time.LoadLocation(timezone(uf)); err == nil {
		return now.In(loc)
	}
	return now.In(time.FixedZone("-03", -3*3600))
}

func timezone(uf string) string {
	switch strings.ToUpper(uf) {
	case "AM", "RR", "RO":
		return "America/Manaus"
	case "MT", "MS":
		return "America/Cuiaba"
	case "AC":
		return "America/Rio_Branco"
	}
	return "America/Sao_Paulo"
}

func (s *Service) sendEvent(ctx context.Context, inv *entity.ServiceInvoice, ev *entity.Event, cert *certificate.Certificate) (*entity.Event, error) {
	res, err := s.gw.SendEvent(ctx, cert, env(inv.Environment), inv.AccessKey, ev.SignedXML)
	now := s.now()
	switch {
	case errors.Is(err, nfse.ErrCertificateRejected):
		ev.Settle(entity.EventNotSent, "", err.Error(), now)
	case errors.Is(err, nfse.ErrUncertain):
		// Não reenvia agora: logo depois do envio a Sefin pode ainda não mostrar o evento.
		// O worker confere e só então reenvia.
		ev.Settle(entity.EventUncertain, "", err.Error(), now)
	case err != nil:
		ev.Settle(entity.EventRejected, "", err.Error(), now)
	case res.Registered():
		ev.EventXML = res.EventXML
		return s.registered(ctx, inv, ev, "")
	case res.AlreadyLinked():
		return s.alreadyLinked(ctx, inv, ev, cert)
	default:
		code, msg := "", ""
		if len(res.Errors) > 0 {
			e := res.Errors[0]
			code, msg = e.Code, strings.TrimSpace(e.Description+" "+e.Complement)
		}
		ev.Settle(entity.EventRejected, code, msg, now)
	}
	if err := s.repo.UpdateEvent(ctx, ev); err != nil {
		return nil, err
	}
	return ev, nil
}

// registered grava o evento e o efeito dele na nota.
func (s *Service) registered(ctx context.Context, inv *entity.ServiceInvoice, ev *entity.Event, msg string) (*entity.Event, error) {
	now := s.now()
	ev.Settle(entity.EventRegistered, "", msg, now)
	if err := s.repo.UpdateEvent(ctx, ev); err != nil {
		return nil, err
	}
	if ev.Kind == entity.KindCancel {
		inv.Cancel(now)
	} else {
		inv.CancelState, inv.UpdatedAt = entity.CancelAnalysis, now
	}
	if err := s.repo.Update(ctx, inv); err != nil {
		return nil, err
	}
	return ev, nil
}

// alreadyLinked trata o E0840. Ele só diz que há ALGUM evento de cancelamento na nota — pode
// ser o pedido de análise. Quem decide é a lista de eventos da nota na Sefin.
func (s *Service) alreadyLinked(ctx context.Context, inv *entity.ServiceInvoice, ev *entity.Event, cert *certificate.Certificate) (*entity.Event, error) {
	state, err := s.gw.CancelState(ctx, cert, env(inv.Environment), inv.AccessKey)
	now := s.now()
	switch {
	case err != nil || state == nfse.CancelStateNone:
		ev.Settle(entity.EventUncertain, nfse.ErrAlreadyLinked, "a Sefin diz que já há evento de cancelamento, mas a lista de eventos não confirma; nova conferência na próxima rodada", now)
	case state == nfse.CancelStateCanceled:
		if ev.Kind == entity.KindCancel {
			return s.registered(ctx, inv, ev, "a NFS-e já estava cancelada")
		}
		inv.Cancel(now)
		if err := s.repo.Update(ctx, inv); err != nil {
			return nil, err
		}
		ev.Settle(entity.EventRejected, nfse.ErrAlreadyLinked, "a NFS-e já está cancelada", now)
	case state == nfse.CancelStateAnalysis:
		if ev.Kind == entity.KindAnalysis {
			return s.registered(ctx, inv, ev, "o pedido de análise já estava registrado")
		}
		ev.Settle(entity.EventRejected, nfse.ErrAlreadyLinked, "há um pedido de análise fiscal aguardando o município; a nota segue válida até a decisão", now)
	case state == nfse.CancelStateDenied:
		ev.Settle(entity.EventRejected, nfse.ErrAlreadyLinked, "o município negou o cancelamento por análise fiscal; a nota segue válida", now)
	}
	if err := s.repo.UpdateEvent(ctx, ev); err != nil {
		return nil, err
	}
	return ev, nil
}

// reconcileEvent confere na Sefin se o evento sem resposta foi registrado; se não foi, e só
// então, reenvia o MESMO pedido.
func (s *Service) reconcileEvent(ctx context.Context, inv *entity.ServiceInvoice, ev *entity.Event, cert *certificate.Certificate) (*entity.Event, error) {
	ok, err := s.gw.EventRegistered(ctx, cert, env(inv.Environment), inv.AccessKey, ev.Kind)
	if err != nil {
		return ev, nil // continua incerto
	}
	if ok {
		return s.registered(ctx, inv, ev, "confirmado na consulta à Sefin")
	}
	return s.sendEvent(ctx, inv, ev, cert)
}

// Events lista os eventos da nota.
func (s *Service) Events(ctx context.Context, tenantID, key string) ([]*entity.Event, error) {
	inv, err := s.repo.FindByKey(ctx, tenantID, key)
	if err != nil {
		return nil, err
	}
	return s.repo.ListEvents(ctx, tenantID, inv.ID)
}

// SweepResult resume uma rodada do worker.
type SweepResult struct {
	Documents, Events, Analyses int
	Errors                      []string
}

// Sweep resolve o que ficou sem resposta: DPS e eventos incertos parados há mais de
// settle (a Sefin demora a mostrar o que acabou de registrar), e as notas em análise
// fiscal, cuja decisão do município chega como evento na nota.
func (s *Service) Sweep(ctx context.Context, settle time.Duration, limit int) SweepResult {
	var res SweepResult
	before := s.now().Add(-settle)
	certs := map[string]*certificate.Certificate{}
	certFor := func(tenant, issuer string) (*certificate.Certificate, error) {
		if c, ok := certs[tenant+"/"+issuer]; ok {
			return c, nil
		}
		c, err := s.signers.SignerForNFSe(ctx, tenant, issuer)
		if err == nil {
			certs[tenant+"/"+issuer] = c
		}
		return c, err
	}
	fail := func(err error) { res.Errors = append(res.Errors, err.Error()) }

	docs, err := s.repo.Pending(ctx, before, limit)
	if err != nil {
		fail(err)
	}
	for _, inv := range docs {
		cert, err := certFor(inv.TenantID, inv.IssuerRef)
		if err != nil {
			fail(err)
			continue
		}
		if out, err := s.reconcile(ctx, inv, cert, true); err != nil {
			fail(err)
		} else if out.Status != entity.StatusUncertain {
			res.Documents++
		}
	}

	events, err := s.repo.PendingEvents(ctx, before, limit)
	if err != nil {
		fail(err)
	}
	for _, ev := range events {
		inv, err := s.repo.FindByID(ctx, ev.TenantID, ev.DocumentID)
		if err != nil {
			fail(err)
			continue
		}
		cert, err := certFor(inv.TenantID, inv.IssuerRef)
		if err != nil {
			fail(err)
			continue
		}
		if out, err := s.reconcileEvent(ctx, inv, ev, cert); err != nil {
			fail(err)
		} else if out.Status != entity.EventUncertain {
			res.Events++
		}
	}

	analyses, err := s.repo.InAnalysis(ctx, limit)
	if err != nil {
		fail(err)
	}
	for _, inv := range analyses {
		cert, err := certFor(inv.TenantID, inv.IssuerRef)
		if err != nil {
			fail(err)
			continue
		}
		state, err := s.gw.CancelState(ctx, cert, env(inv.Environment), inv.AccessKey)
		if err != nil {
			continue
		}
		switch state {
		case nfse.CancelStateCanceled:
			inv.Cancel(s.now())
		case nfse.CancelStateDenied:
			inv.CancelState, inv.UpdatedAt = entity.CancelDenied, s.now()
		default:
			continue
		}
		if err := s.repo.Update(ctx, inv); err != nil {
			fail(err)
			continue
		}
		res.Analyses++
	}
	return res
}
