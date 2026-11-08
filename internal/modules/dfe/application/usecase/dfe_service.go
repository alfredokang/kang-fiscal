// Package usecase orquestra o recebimento de notas e a manifestação do destinatário.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// DFeService é a fachada do módulo.
type DFeService struct {
	repo    output.Repository
	issuers output.IssuerReader
	signers output.SignerProvider
	gw      output.Gateway
	now     func() time.Time
}

// NewDFeService monta o serviço.
func NewDFeService(repo output.Repository, issuers output.IssuerReader, signers output.SignerProvider, gw output.Gateway) *DFeService {
	return &DFeService{repo: repo, issuers: issuers, signers: signers, gw: gw, now: func() time.Time { return time.Now().UTC() }}
}

// maxBatches é quantos lotes de 50 uma sincronização baixa de uma vez: um CNPJ com
// histórico grande termina nas rodadas seguintes, sem prender o worker.
const maxBatches = 20

func env(e string) sefaz.Environment {
	if e == "production" {
		return sefaz.Production
	}
	return sefaz.Homologation
}

// SyncResult resume uma sincronização.
type SyncResult struct {
	Received    int
	CStat       int
	Motive      string
	NextQueryAt time.Time
}

// Sync baixa os documentos novos do emitente, respeitando a espera que a SEFAZ exige.
func (s *DFeService) Sync(ctx context.Context, tenantID, issuerRef string) (SyncResult, error) {
	issuer, err := s.issuers.IssuerForDFe(ctx, tenantID, issuerRef)
	if err != nil {
		return SyncResult{}, err
	}
	cur, err := s.repo.Cursor(ctx, tenantID, issuerRef)
	if err != nil {
		return SyncResult{}, err
	}
	if s.now().Before(cur.NextQueryAt) {
		return SyncResult{CStat: cur.LastCStat, NextQueryAt: cur.NextQueryAt}, domain.ErrTooSoon
	}
	cert, err := s.signers.SignerForDFe(ctx, tenantID, issuerRef)
	if err != nil {
		return SyncResult{}, err
	}
	var out SyncResult
	for range maxBatches {
		r, err := s.gw.Distribute(ctx, cert, sefaz.DistributionQuery{Environment: env(issuer.Environment),
			UFCode: sefaz.UFCode(issuer.UF), CNPJ: issuer.CNPJ, LastNSU: cur.LastNSU})
		if err != nil {
			return out, err // cursor intacto: a próxima rodada pede do mesmo ponto
		}
		for _, d := range r.Docs {
			if err := s.store(ctx, tenantID, issuerRef, d); err != nil {
				return out, err
			}
			out.Received++
		}
		more := cur.After(r.CStat, r.LastNSU, r.MaxNSU, s.now())
		out.CStat, out.Motive, out.NextQueryAt = r.CStat, r.Motive, cur.NextQueryAt
		if err := s.repo.SaveCursor(ctx, cur); err != nil {
			return out, err
		}
		if !more {
			break
		}
	}
	return out, nil
}

func (s *DFeService) store(ctx context.Context, tenantID, issuerRef string, d sefaz.DistributedDoc) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	doc := &entity.Document{ID: id.String(), TenantID: tenantID, IssuerRef: issuerRef, NSU: d.NSU, Schema: d.Schema,
		Kind: entity.KindOf(d.Schema), XML: d.XML, CreatedAt: s.now()}
	doc.Describe()
	return s.repo.SaveDocument(ctx, doc)
}

// SyncDue sincroniza os emitentes cuja espera já passou (o worker chama).
func (s *DFeService) SyncDue(ctx context.Context, limit int) (synced, received int, errs []string) {
	due, err := s.repo.DueCursors(ctx, limit)
	if err != nil {
		return 0, 0, []string{err.Error()}
	}
	for _, c := range due {
		r, err := s.Sync(ctx, c.TenantID, c.IssuerRef)
		if err != nil && !errors.Is(err, domain.ErrTooSoon) {
			errs = append(errs, err.Error())
			continue
		}
		synced++
		received += r.Received
	}
	return synced, received, errs
}

// Manifest registra a manifestação do destinatário sobre uma nota recebida.
//
// Depois da ciência (ou da confirmação), a SEFAZ libera o XML completo: ele é baixado na
// hora, pela chave, e entra na lista como documento "full".
func (s *DFeService) Manifest(ctx context.Context, tenantID, docID, tpEvento, justification string) (sefaz.EventResult, error) {
	doc, err := s.repo.Find(ctx, tenantID, docID)
	if err != nil {
		return sefaz.EventResult{}, err
	}
	if doc.Kind != entity.KindSummary && doc.Kind != entity.KindFull {
		return sefaz.EventResult{}, domain.ErrNotSummary
	}
	issuer, err := s.issuers.IssuerForDFe(ctx, tenantID, doc.IssuerRef)
	if err != nil {
		return sefaz.EventResult{}, err
	}
	cert, err := s.signers.SignerForDFe(ctx, tenantID, doc.IssuerRef)
	if err != nil {
		return sefaz.EventResult{}, err
	}
	tpAmb := 2
	if issuer.Environment == "production" {
		tpAmb = 1
	}
	ev, err := nfe.NewManifestation(nfe.EventParty{TpAmb: tpAmb, CNPJ: issuer.CNPJ, Key: doc.AccessKey,
		When: s.now().In(time.FixedZone("-03", -3*3600))}, tpEvento, justification)
	if err != nil {
		return sefaz.EventResult{}, fmt.Errorf("%w: %v", domain.ErrInvalidRequest, err)
	}
	raw, err := nfe.MarshalEvent(ev)
	if err != nil {
		return sefaz.EventResult{}, err
	}
	signed, err := xmldsig.Sign(raw, "infEvento", cert)
	if err != nil {
		return sefaz.EventResult{}, err
	}
	res, err := s.gw.SendEvent(ctx, cert, sefaz.Target{Model: sefaz.NFe, UF: "AN", Environment: env(issuer.Environment)},
		time.Now().UnixMilli(), signed)
	if err != nil {
		return res, err
	}
	// 573 é duplicidade: a manifestação já estava registrada.
	if res.Registered() || res.CStat == 573 {
		if err := s.repo.SetManifestation(ctx, tenantID, doc.AccessKey, tpEvento); err != nil {
			return res, err
		}
		if tpEvento == nfe.ManifestAwareness || tpEvento == nfe.ManifestConfirmation {
			if full, err := s.gw.Distribute(ctx, cert, sefaz.DistributionQuery{Environment: env(issuer.Environment),
				UFCode: sefaz.UFCode(issuer.UF), CNPJ: issuer.CNPJ, AccessKey: doc.AccessKey}); err == nil {
				for _, d := range full.Docs {
					_ = s.store(ctx, tenantID, doc.IssuerRef, d)
				}
			}
		}
	}
	return res, nil
}

// List lista os documentos recebidos.
func (s *DFeService) List(ctx context.Context, tenantID, issuerRef string, page pagination.Params) ([]*entity.Document, int64, error) {
	return s.repo.List(ctx, tenantID, issuerRef, page)
}

// Get lê um documento.
func (s *DFeService) Get(ctx context.Context, tenantID, id string) (*entity.Document, error) {
	return s.repo.Find(ctx, tenantID, id)
}
