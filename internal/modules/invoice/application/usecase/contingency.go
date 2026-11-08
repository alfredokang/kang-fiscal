package usecase

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/service"
	"github.com/alfredokang/kang-fiscal/pkg/accesskey"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

// emissionTarget é o destino do envio e da consulta de uma nota: a nota emitida em SVC
// (tpEmis 6 ou 7, que está na chave) só existe na SVC.
func emissionTarget(inv *entity.Invoice, uf string) sefaz.Target {
	t := target(inv, uf)
	switch inv.AccessKey[34] {
	case '6':
		t.Contingency = sefaz.SVCAN
	case '7':
		t.Contingency = sefaz.SVCRS
	}
	return t
}

// eventTarget é o destino dos eventos: na SVC enquanto a contingência estiver ativa; de
// volta à UF quando ela acabar (a SVC repassa as notas para a UF).
func (s *InvoiceService) eventTarget(ctx context.Context, inv *entity.Invoice, uf string) sefaz.Target {
	t := emissionTarget(inv, uf)
	if t.Contingency != sefaz.NoContingency {
		if c, err := s.d.Contingency.Get(ctx, uf, inv.Model, inv.Environment); err != nil || !c.Active {
			t.Contingency = sefaz.NoContingency
		}
	}
	return t
}

// normalEmission diz se a nota é de emissão normal de NF-e — a única que entra na conta
// das falhas e que pode ir para a SVC.
func normalEmission(inv *entity.Invoice) bool { return inv.Model == 55 && inv.AccessKey[34] == '1' }

// recordFailure conta uma falha de conexão com a autorizadora normal e liga a SVC na
// terceira falha da janela.
func (s *InvoiceService) recordFailure(ctx context.Context, inv *entity.Invoice, uf string, cause error) {
	n, err := s.d.Contingency.RecordFailure(ctx, uf, inv.Model, inv.Environment, s.d.Now(), entity.FailureWindow)
	if err != nil || n < entity.FailuresToActivate {
		return
	}
	reason := fmt.Sprintf("SEFAZ %s sem resposta em %d tentativas seguidas", uf, n)
	_ = s.activate(ctx, uf, inv.Model, inv.Environment, reason)
	_ = cause
}

func (s *InvoiceService) activate(ctx context.Context, uf string, model int, env, reason string) error {
	kind := string(sefaz.ContingencyFor(uf))
	if kind == "" {
		return fmt.Errorf("%w: UF %s sem SVC", domain.ErrInvalidRequest, uf)
	}
	return s.d.Contingency.Activate(ctx, entity.Contingency{UF: strings.ToUpper(uf), Model: model, Environment: env,
		Active: true, Kind: kind, Reason: reason, StartedAt: s.d.Now()})
}

// reissueInContingency reassina a nota para a SVC: mesmo número, tpEmis 6 ou 7, dhCont e
// xJust, e por isso chave nova. Só é chamado quando a SEFAZ recusou o lote por estar parada
// (108/109) — a nota com certeza não foi autorizada, e o número continua livre.
func (s *InvoiceService) reissueInContingency(ctx context.Context, inv *entity.Invoice, cert *certificate.Certificate, uf string) (*entity.Invoice, error) {
	c, err := s.d.Contingency.Get(ctx, uf, inv.Model, inv.Environment)
	if err != nil || !c.Active {
		return inv, err
	}
	var doc nfe.NFe
	if err := xml.Unmarshal(inv.SignedXML, &doc); err != nil {
		return nil, err
	}
	ide := &doc.InfNFe.Ide
	service.ApplyContingency(ide, c, uf)
	issued, err := time.Parse("2006-01-02T15:04:05-07:00", ide.DhEmi)
	if err != nil {
		return nil, err
	}
	code := 0
	for _, r := range ide.CNF {
		code = code*10 + int(r-'0')
	}
	key, err := accesskey.Build(accesskey.Parts{UFCode: ide.CUF, Issued: issued, Document: doc.InfNFe.Emit.CNPJ,
		Model: accesskey.Model(ide.Mod), Series: ide.Serie, Number: ide.NNF, EmissionType: ide.TpEmis, Code: code})
	if err != nil {
		return nil, err
	}
	k := key.String()
	ide.CDV = int(k[43] - '0')
	doc.InfNFe.ID = "NFe" + k
	raw, err := nfe.Marshal(&doc)
	if err != nil {
		return nil, err
	}
	signed, err := xmldsig.Sign(raw, "infNFe", cert)
	if err != nil {
		return nil, err
	}
	inv.AccessKey, inv.SignedXML, inv.UpdatedAt = k, signed, s.d.Now()
	inv.Settle(entity.StatusSigned, 0, "reemitida em contingência ("+c.Kind+")", inv.UpdatedAt)
	if err := s.d.Repo.UpdateDocument(ctx, inv); err != nil {
		return nil, err
	}
	if err := s.d.Repo.Update(ctx, inv); err != nil {
		return nil, err
	}
	return s.send(ctx, inv, cert, uf, false)
}

// ActivateContingency liga a SVC à mão (a SEFAZ publicou parada programada, por exemplo).
func (s *InvoiceService) ActivateContingency(ctx context.Context, uf string, model int, env, reason string) (entity.Contingency, error) {
	if model != 55 {
		return entity.Contingency{}, fmt.Errorf("%w: a SVC é só da NF-e; a NFC-e usa a emissão offline", domain.ErrInvalidRequest)
	}
	if n := len([]rune(strings.TrimSpace(reason))); n < 15 || n > 256 {
		return entity.Contingency{}, fmt.Errorf("%w: motivo da contingência com 15 a 256 caracteres", domain.ErrInvalidRequest)
	}
	if err := s.activate(ctx, uf, model, env, reason); err != nil {
		return entity.Contingency{}, err
	}
	return s.d.Contingency.Get(ctx, strings.ToUpper(uf), model, env)
}

// DeactivateContingency volta a UF ao normal.
func (s *InvoiceService) DeactivateContingency(ctx context.Context, uf string, model int, env string) error {
	return s.d.Contingency.Deactivate(ctx, strings.ToUpper(uf), model, env)
}

// Contingencies lista as contingências ativas.
func (s *InvoiceService) Contingencies(ctx context.Context) ([]entity.Contingency, error) {
	return s.d.Contingency.ListActive(ctx)
}

// ProbeContingencies testa, para cada contingência ativa, se a autorizadora normal voltou
// (status 107) e desliga a SVC. Usa o certificado do emitente da nota mais recente da UF:
// o status do serviço aceita qualquer certificado válido.
func (s *InvoiceService) ProbeContingencies(ctx context.Context) (restored []string) {
	active, err := s.d.Contingency.ListActive(ctx)
	if err != nil {
		return nil
	}
	for _, c := range active {
		cuf := fmt.Sprintf("%02d", sefaz.UFCode(c.UF))
		tenant, issuerRef, err := s.d.Repo.LatestIssuer(ctx, cuf, c.Model, c.Environment)
		if err != nil {
			continue
		}
		cert, err := s.d.Signers.SignerFor(ctx, tenant, issuerRef)
		if err != nil {
			continue
		}
		env := sefaz.Homologation
		if c.Environment == "production" {
			env = sefaz.Production
		}
		st, err := s.d.Sefaz.Status(ctx, cert, sefaz.Target{Model: sefaz.Model(c.Model), UF: c.UF, Environment: env})
		if err == nil && st.Online() {
			if s.d.Contingency.Deactivate(ctx, c.UF, c.Model, c.Environment) == nil {
				restored = append(restored, c.UF)
			}
		}
	}
	return restored
}
