package usecase

import (
	"context"
	"encoding/xml"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/accesskey"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfce"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

// OfflineJustification é o xJust das NFC-e em contingência offline.
const OfflineJustification = "Falha de comunicacao com a SEFAZ autorizadora"

// onlineNFCe diz se a nota é NFC-e de emissão normal — a única que pode ir para o offline.
func onlineNFCe(inv *entity.Invoice) bool { return inv.Model == 65 && inv.AccessKey[34] == '1' }

// goOffline cobre a venda de uma NFC-e online sem resposta com uma NFC-e em contingência
// offline (tpEmis 9): NOVO número, QR Code v3 assinado, pronta para imprimir agora.
//
// A online fica marcada como substituída e nunca é reenviada: ela pode ter sido
// autorizada. O worker (TransmitOffline) resolve o destino dela depois.
func (s *InvoiceService) goOffline(ctx context.Context, online *entity.Invoice, cert *certificate.Certificate, uf string) (*entity.Invoice, error) {
	var doc nfe.NFe
	if err := xml.Unmarshal(online.SignedXML, &doc); err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	resRef, number, err := s.d.Numbers.ReserveNumber(ctx, output.NumberKey{TenantID: online.TenantID, IssuerRef: online.IssuerRef,
		Environment: online.Environment, Model: online.Model, Series: online.Series}, id.String())
	if err != nil {
		return nil, err
	}
	code, err := accesskey.NewCode(number)
	if err != nil {
		return nil, err
	}
	now := s.d.Now()
	ide := &doc.InfNFe.Ide
	issued, err := time.Parse("2006-01-02T15:04:05-07:00", ide.DhEmi)
	if err != nil {
		return nil, err
	}
	ide.NNF, ide.TpEmis = number, 9
	ide.DhCont = now.In(issued.Location()).Format("2006-01-02T15:04:05-07:00")
	ide.XJust = OfflineJustification
	key, err := accesskey.Build(accesskey.Parts{UFCode: ide.CUF, Issued: issued, Document: doc.InfNFe.Emit.CNPJ,
		Model: accesskey.NFCe, Series: ide.Serie, Number: number, EmissionType: 9, Code: code})
	if err != nil {
		return nil, err
	}
	k := key.String()
	ide.CNF, ide.CDV, doc.InfNFe.ID = k[35:43], int(k[43]-'0'), "NFe"+k

	off := nfce.Offline{AccessKey: k, TpAmb: ide.TpAmb, Day: issued.Day(), VNF: doc.InfNFe.Total.ICMSTot.VNF.String()}
	if d := doc.InfNFe.Dest; d != nil {
		off.CNPJ, off.CPF = d.CNPJ, d.CPF
	}
	if doc.InfNFeSupl, err = nfce.SupplementOffline(uf, off, cert); err != nil {
		return nil, err
	}
	raw, err := nfe.Marshal(&doc)
	if err != nil {
		return nil, err
	}
	signed, err := xmldsig.Sign(raw, "infNFe", cert)
	if err != nil {
		return nil, err
	}
	inv := &entity.Invoice{ID: id.String(), TenantID: online.TenantID, IssuerRef: online.IssuerRef,
		IdempotencyKey: online.IdempotencyKey + ":offline", RequestHash: online.RequestHash, Model: online.Model,
		Series: online.Series, Number: number, ReservationRef: resRef, Environment: online.Environment, AccessKey: k,
		Status: entity.StatusOffline, TotalAmount: online.TotalAmount, SignedXML: signed, CreatedAt: now, UpdatedAt: now,
		Motive: "emitida em contingência offline; substitui a NFC-e " + online.AccessKey}
	if err := s.d.Repo.Create(ctx, inv); err != nil {
		return nil, err
	}
	online.SupersededBy = inv.ID
	online.UpdatedAt = now
	if err := s.d.Repo.Update(ctx, online); err != nil {
		return nil, err
	}
	return inv, nil
}

// OfflineResult resume uma rodada de transmissão.
type OfflineResult struct {
	Transmitted, Authorized, Substituted, Released int
	Errors                                         []string
}

// TransmitOffline transmite as NFC-e offline e resolve as notas online que elas
// substituíram:
//   - a offline vai para a SEFAZ (prazo: até 24 horas depois da emissão);
//   - a online substituída é consultada. Autorizada → cancelamento por substituição
//     (110112) apontando para a offline. A SEFAZ não a conhece → fica rejeitada, e o
//     número entra na inutilização. Sem resposta → espera a próxima rodada.
func (s *InvoiceService) TransmitOffline(ctx context.Context, limit int) OfflineResult {
	var res OfflineResult
	pending, err := s.d.Repo.Pending(ctx, limit)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res
	}
	for _, inv := range pending {
		issuer, _, err := s.d.Issuers.IssuerForInvoice(ctx, inv.TenantID, inv.IssuerRef)
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		cert, err := s.d.Signers.SignerFor(ctx, inv.TenantID, inv.IssuerRef)
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		uf := issuer.Address.UF
		if inv.Status == entity.StatusOffline {
			res.Transmitted++
			out, err := s.send(ctx, inv, cert, uf, false)
			if err == nil && out.Status == entity.StatusAuthorized {
				res.Authorized++
			}
			continue
		}
		if err := s.settleSuperseded(ctx, inv, cert, uf, &res); err != nil {
			res.Errors = append(res.Errors, err.Error())
		}
	}
	return res
}

func (s *InvoiceService) settleSuperseded(ctx context.Context, online *entity.Invoice, cert *certificate.Certificate, uf string, res *OfflineResult) error {
	q, err := s.d.Sefaz.Query(ctx, cert, emissionTarget(online, uf), online.AccessKey)
	if err != nil {
		return nil // sem resposta: próxima rodada
	}
	switch {
	case q.Protocol != nil && entity.IsAuthorization(q.Protocol.CStat):
		// A online saiu autorizada: a venda tem duas notas. Registra a autorização e
		// cancela por substituição, apontando a offline.
		if err := s.apply(ctx, online, q.Protocol); err != nil {
			return err
		}
		replacement, err := s.d.Repo.FindByID(ctx, online.TenantID, online.SupersededBy)
		if err != nil {
			return err
		}
		party, err := s.party(online)
		if err != nil {
			return err
		}
		ev, err := nfe.NewSubstitution(party, online.Protocol, "Venda coberta por NFC-e emitida em contingencia offline",
			replacement.AccessKey, "kang-fiscal")
		if err != nil {
			return err
		}
		raw, err := nfe.MarshalSubstitution(ev)
		if err != nil {
			return err
		}
		e, err := s.createAndSendRaw(ctx, online, raw, nfe.EventSubstitution, 1, "substituída pela NFC-e "+replacement.AccessKey, cert)
		if err != nil {
			return err
		}
		if e.Status == entity.EventRegistered {
			res.Substituted++
		}
	case q.Protocol != nil && entity.IsDenial(q.Protocol.CStat):
		return s.apply(ctx, online, q.Protocol)
	case q.CStat == 217:
		// A SEFAZ não a conhece: não existe. O número fica livre e a varredura o inutiliza.
		online.Settle(entity.StatusRejected, 217, "não recebida pela SEFAZ; venda coberta pela NFC-e offline", s.d.Now())
		if err := s.d.Repo.Update(ctx, online); err != nil {
			return err
		}
		res.Released++
	}
	return nil
}
