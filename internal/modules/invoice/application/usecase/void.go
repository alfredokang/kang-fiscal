package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// AutoVoidJustification é a justificativa das inutilizações automáticas.
const AutoVoidJustification = "Numeracao nao utilizada: nota rejeitada ou nao emitida por falha no envio"

// VoidCommand é a inutilização manual de uma faixa.
type VoidCommand struct {
	TenantID, IssuerRef string
	Model, Series       int
	From, To            int
	Justification       string
}

// VoidRange inutiliza uma faixa informada por quem opera.
//
// Recusa a faixa que contenha nota autorizada, denegada, cancelada ou ainda incerta: a
// incerta pode ter sido autorizada, e inutilizar o número dela seria pior que o buraco.
func (s *InvoiceService) VoidRange(ctx context.Context, cmd VoidCommand) (*entity.Void, error) {
	issuer, _, err := s.d.Issuers.IssuerForInvoice(ctx, cmd.TenantID, cmd.IssuerRef)
	if err != nil {
		return nil, err
	}
	key := output.NumberKey{TenantID: cmd.TenantID, IssuerRef: cmd.IssuerRef, Environment: issuer.Environment,
		Model: cmd.Model, Series: cmd.Series}
	if cmd.To-cmd.From > 999 {
		return nil, fmt.Errorf("%w: no máximo 1.000 números por pedido", domain.ErrInvalidRequest)
	}
	invs, err := s.d.Repo.InRange(ctx, key, cmd.From, cmd.To)
	if err != nil {
		return nil, err
	}
	for _, inv := range invs {
		if inv.Status != entity.StatusRejected && inv.Status != entity.StatusNotSent {
			return nil, fmt.Errorf("%w: o número %d é da nota %s, que está %s", domain.ErrInvalidRequest, inv.Number, inv.AccessKey, inv.Status)
		}
	}
	return s.voidRange(ctx, key, issuer.CNPJ, issuer.Address.UF, cmd.From, cmd.To, cmd.Justification, nil)
}

// SweepResult resume uma varredura.
type SweepResult struct {
	Checked, Voided, Skipped int
	Errors                   []string
}

// SweepPending inutiliza os buracos: números reservados há mais de `age` sem nota válida.
//
// Para cada número aberto:
//   - nota rejeitada ou não enviada → entra na inutilização;
//   - nota assinada ou incerta → consulta a SEFAZ antes; se sair autorizada, o número é
//     dela; se continuar sem resposta, espera a próxima varredura;
//   - nota autorizada ou denegada (reserva que ficou aberta por queda) → fecha como usado;
//   - nenhuma nota (queda entre a reserva e a gravação) → entra na inutilização.
//
// Números seguidos da mesma série viram uma faixa só: um pedido por buraco, não por número.
func (s *InvoiceService) SweepPending(ctx context.Context, age time.Duration, limit int) SweepResult {
	var res SweepResult
	pending, err := s.d.Numbers.PendingNumbers(ctx, s.d.Now().Add(-age), limit)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res
	}
	type group struct {
		key  output.NumberKey
		nums []output.PendingNumber
	}
	groups := map[string]*group{}
	for _, p := range pending {
		if p.Key.Model == 99 {
			continue // DPS da NFS-e: não existe inutilização; o módulo nfse fecha a reserva
		}
		res.Checked++
		inv, err := s.d.Repo.FindByID(ctx, p.Key.TenantID, p.DocumentRef)
		switch {
		case errors.Is(err, domain.ErrNotFound):
			// sem nota: buraco
		case err != nil:
			res.Errors = append(res.Errors, err.Error())
			continue
		default:
			if inv.SupersededBy != "" && (inv.Status == entity.StatusSigned || inv.Status == entity.StatusUncertain) {
				res.Skipped++ // substituída por NFC-e offline: o destino é do TransmitOffline
				continue
			}
			if inv.Status == entity.StatusOffline {
				res.Skipped++ // o número é da NFC-e offline, a transmitir
				continue
			}
			if inv.Status == entity.StatusSigned || inv.Status == entity.StatusUncertain {
				if inv, err = s.resume(ctx, inv); err != nil {
					res.Errors = append(res.Errors, err.Error())
					continue
				}
			}
			switch inv.Status {
			case entity.StatusAuthorized, entity.StatusDenied, entity.StatusCanceled:
				_ = s.d.Numbers.MarkNumberUsed(ctx, p.Key.TenantID, p.ReservationRef)
				res.Skipped++
				continue
			case entity.StatusSigned, entity.StatusUncertain:
				res.Skipped++ // ainda sem resposta: nunca inutilizar
				continue
			}
		}
		k := fmt.Sprintf("%s|%s|%s|%d|%d", p.Key.TenantID, p.Key.IssuerRef, p.Key.Environment, p.Key.Model, p.Key.Series)
		if groups[k] == nil {
			groups[k] = &group{key: p.Key}
		}
		groups[k].nums = append(groups[k].nums, p)
	}

	for _, g := range groups {
		issuer, _, err := s.d.Issuers.IssuerForInvoice(ctx, g.key.TenantID, g.key.IssuerRef)
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		sort.Slice(g.nums, func(i, j int) bool { return g.nums[i].Number < g.nums[j].Number })
		for start := 0; start < len(g.nums); {
			end := start
			for end+1 < len(g.nums) && g.nums[end+1].Number == g.nums[end].Number+1 {
				end++
			}
			var refs []string
			for _, n := range g.nums[start : end+1] {
				refs = append(refs, n.ReservationRef)
			}
			v, err := s.voidRange(ctx, g.key, issuer.CNPJ, issuer.Address.UF, g.nums[start].Number, g.nums[end].Number, AutoVoidJustification, refs)
			switch {
			case err != nil:
				res.Errors = append(res.Errors, err.Error())
			case v.Status == entity.VoidDone:
				res.Voided += len(refs)
			default:
				res.Errors = append(res.Errors, fmt.Sprintf("faixa %d-%d: %d %s", v.From, v.To, v.CStat, v.Motive))
			}
			start = end + 1
		}
	}
	return res
}

func (s *InvoiceService) voidRange(ctx context.Context, key output.NumberKey, cnpj, uf string, from, to int, justification string, refs []string) (*entity.Void, error) {
	cert, err := s.d.Signers.SignerFor(ctx, key.TenantID, key.IssuerRef)
	if err != nil {
		return nil, err
	}
	tpAmb := 2
	if key.Environment == "production" {
		tpAmb = 1
	}
	cuf := sefaz.UFCode(uf)
	req, err := nfe.NewVoidRange(nfe.VoidRange{TpAmb: tpAmb, CUF: cuf, Model: key.Model, Series: key.Series,
		Year: s.d.Now().Year(), CNPJ: cnpj, From: from, To: to, Justification: justification})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidRequest, err)
	}
	raw, err := nfe.MarshalVoid(req)
	if err != nil {
		return nil, err
	}
	signed, err := xmldsig.Sign(raw, "infInut", cert)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := s.d.Now()
	v := &entity.Void{ID: id.String(), TenantID: key.TenantID, IssuerRef: key.IssuerRef, Environment: key.Environment,
		Model: key.Model, Series: key.Series, From: from, To: to, Justification: req.InfInut.XJust,
		ReservationRefs: refs, Status: entity.EventSigned, SignedXML: signed, CreatedAt: now, UpdatedAt: now}
	if err := s.d.Voids.CreateVoid(ctx, v); err != nil {
		return nil, err
	}

	env := sefaz.Homologation
	if tpAmb == 1 {
		env = sefaz.Production
	}
	r, err := s.d.Sefaz.VoidRange(ctx, cert, sefaz.Target{Model: sefaz.Model(key.Model), UF: uf, Environment: env}, signed)
	v.UpdatedAt = s.d.Now()
	switch {
	case errors.Is(err, sefaz.ErrCertificateRejected), errors.Is(err, sefaz.ErrServerUntrusted):
		v.Status, v.Motive = entity.EventNotSent, err.Error()
	case err != nil:
		// Repetir uma inutilização é seguro: a SEFAZ responde que a faixa já está
		// inutilizada. Fica incerta e a próxima varredura pede de novo.
		v.Status, v.Motive = entity.EventUncertain, err.Error()
	case r.Voided():
		v.Status, v.CStat, v.Motive, v.Protocol = entity.VoidDone, r.CStat, r.Motive, r.Protocol
		v.ProcXML = nfe.VoidProc(signed, r.RetInut)
	default:
		v.Status, v.CStat, v.Motive = entity.EventRejected, r.CStat, r.Motive
	}
	if err := s.d.Voids.UpdateVoid(ctx, v); err != nil {
		return nil, err
	}
	if v.Status == entity.VoidDone {
		for _, ref := range refs {
			if err := s.d.Numbers.MarkNumberVoided(ctx, key.TenantID, ref); err != nil {
				return v, err
			}
		}
	}
	return v, nil
}

// Voids lista as inutilizações.
func (s *InvoiceService) Voids(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.Void, int64, error) {
	return s.d.Voids.ListVoids(ctx, tenantID, page)
}
