package nfe_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

var party = nfe.EventParty{OrgCode: 35, TpAmb: 2, CNPJ: nfetest.CNPJ,
	Key:  "35261111222333000181550010000001231123456787",
	When: time.Date(2026, 11, 8, 11, 0, 0, 0, time.FixedZone("-03", -3*3600))}

func TestCancelamentoPassaNoXSD(t *testing.T) {
	ev, err := nfe.NewCancellation(party, "135260000000001", "  Pedido   cancelado pelo cliente antes do envio ")
	if err != nil {
		t.Fatal(err)
	}
	if ev.InfEvento.ID != "ID110111"+party.Key+"01" || ev.InfEvento.DetEvento.XJust != "Pedido cancelado pelo cliente antes do envio" {
		t.Errorf("evento: %+v", ev.InfEvento)
	}
	raw, _ := nfe.MarshalEvent(ev)
	signed := nfetest.SignEvent(t, raw)
	batch := nfe.EventBatch(42, signed)
	if _, err := xmldsig.Verify(batch); err != nil {
		t.Fatalf("a assinatura não sobrevive ao envelope do lote: %v", err)
	}
	nfetest.MustValidateSchema(t, batch, "envEventoCancNFe_v1.00.xsd")
}

func TestCartaDeCorrecaoPassaNoXSD(t *testing.T) {
	ev, err := nfe.NewCorrection(party, 2, "Onde se le RUA DAS FLORES 10, leia-se RUA DAS FLORES 100")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(ev.InfEvento.ID, "02") || ev.InfEvento.DetEvento.XCondUso != nfe.CorrectionTerms {
		t.Errorf("CC-e: %+v", ev.InfEvento)
	}
	raw, _ := nfe.MarshalEvent(ev)
	nfetest.MustValidateSchema(t, nfe.EventBatch(1, nfetest.SignEvent(t, raw)), "envCCe_v1.00.xsd")
}

func TestEventoRecusa(t *testing.T) {
	if _, err := nfe.NewCancellation(party, "1", "curto"); !errors.Is(err, nfe.ErrJustification) {
		t.Errorf("justificativa curta: %v", err)
	}
	if _, err := nfe.NewCorrection(party, 21, strings.Repeat("x", 20)); !errors.Is(err, nfe.ErrSequence) {
		t.Errorf("21ª correção: %v", err)
	}
	if _, err := nfe.NewCorrection(party, 1, strings.Repeat("x", 1001)); !errors.Is(err, nfe.ErrCorrection) {
		t.Errorf("correção longa: %v", err)
	}
}

func TestCancelamentoPorSubstituicaoPassaNoXSD(t *testing.T) {
	p := party
	p.Key = "35261111222333000181650010000000071000000077"
	ev, err := nfe.NewSubstitution(p, "135260000000001", "Venda coberta por NFC-e emitida em contingencia", "35261111222333000181650010000000089123456781", "kang-fiscal")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := nfe.MarshalSubstitution(ev)
	nfetest.MustValidateSchema(t, nfe.EventBatch(1, nfetest.SignEvent(t, raw)), "envEventoCancSubst_v1.00.xsd")
}

func TestManifestacaoPassaNoXSD(t *testing.T) {
	for _, tp := range []string{nfe.ManifestConfirmation, nfe.ManifestAwareness, nfe.ManifestUnknown, nfe.ManifestNotPerformed} {
		ev, err := nfe.NewManifestation(party, tp, "A mercadoria nunca foi entregue no endereco")
		if err != nil {
			t.Fatal(err)
		}
		if ev.InfEvento.COrgao != 91 {
			t.Errorf("%s: cOrgao %d", tp, ev.InfEvento.COrgao)
		}
		raw, _ := nfe.MarshalEvent(ev)
		nfetest.MustValidateSchema(t, nfe.EventBatch(1, nfetest.SignEvent(t, raw)), "envConfRecebto_v1.00.xsd")
	}
	if _, err := nfe.NewManifestation(party, nfe.ManifestNotPerformed, "curta"); err == nil {
		t.Error("operação não realizada sem justificativa")
	}
}
