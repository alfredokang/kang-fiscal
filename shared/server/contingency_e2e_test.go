package server_test

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

// SEFAZ-SP parada (108): a nota é recusada, a SVC-AN é ligada e a MESMA venda sai por lá,
// com o mesmo número e chave nova. As próximas já vão direto para a SVC; quando a SP volta,
// a sonda desliga a contingência e tudo volta ao normal.
func TestContingenciaSVCQuandoASefazPara(t *testing.T) {
	db := openIntegrationDB(t)
	var down atomic.Bool
	down.Store(true)
	f := &fakeSefaz{}
	f.authorize = func(key string) (sefaz.AuthorizeResult, error) {
		if down.Load() && key[34] == '1' {
			return sefaz.AuthorizeResult{BatchCStat: 108, BatchMotive: "Servico Paralisado Momentaneamente"}, nil
		}
		return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 100, "Autorizado o uso da NF-e")}, nil
	}
	f.status = func() (sefaz.StatusResult, error) {
		if down.Load() {
			return sefaz.StatusResult{CStat: 108}, nil
		}
		return sefaz.StatusResult{CStat: 107}, nil
	}
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)

	rec, inv := issue(t, h, "svc-1", issueBody(issuerID, "09012100"))
	if rec.Code != http.StatusCreated || inv["status"] != "authorized" {
		t.Fatalf("emissão com a SP parada: %d %v", rec.Code, inv)
	}
	key := inv["access_key"].(string)
	if key[34] != '6' || inv["number"].(float64) != 1 {
		t.Errorf("a nota deveria sair pela SVC-AN (tpEmis 6) com o número 1: %s %v", key, inv["number"])
	}
	var signed []byte
	_ = db.QueryRow(`SELECT signed_xml FROM invoice_invoices WHERE access_key = $1`, key).Scan(&signed)
	if s := string(signed); !strings.Contains(s, "<tpEmis>6</tpEmis>") || !strings.Contains(s, "<dhCont>") || !strings.Contains(s, "<xJust>SEFAZ SP parada") {
		t.Errorf("XML de contingência sem tpEmis/dhCont/xJust")
	}
	nfetest.MustValidateXSD(t, signed)

	// A próxima vai direto para a SVC, sem tentar a SP antes.
	before := f.authorizeCalls.Load()
	_, inv2 := issue(t, h, "svc-2", issueBody(issuerID, "09012100"))
	if k := inv2["access_key"].(string); k[34] != '6' || f.authorizeCalls.Load() != before+1 {
		t.Errorf("segunda nota: %s, envios %d", k, f.authorizeCalls.Load()-before)
	}
	_, list := do(t, h, http.MethodGet, "/v1/admin/contingency", adminToken, nil)
	if n := len(list["data"].([]any)); n != 1 {
		t.Fatalf("contingências ativas: %d", n)
	}

	// A SP voltou: a sonda desliga a SVC, e a próxima nota sai normal.
	down.Store(false)
	srv, _ := server.NewServer(db, server.WithSefaz(f))
	if restored := srv.Invoices().ProbeContingencies(context.Background()); len(restored) != 1 || restored[0] != "SP" {
		t.Fatalf("sonda: %v", restored)
	}
	if _, inv3 := issue(t, h, "svc-3", issueBody(issuerID, "09012100")); inv3["access_key"].(string)[34] != '1' {
		t.Errorf("depois da volta: %v", inv3["access_key"])
	}
}

// Três falhas de conexão seguidas ligam a SVC — e as notas incertas NÃO são reemitidas:
// elas podem ter sido autorizadas pela UF, e a SVC daria outra chave à mesma venda.
func TestContingenciaPorFalhasNaoReemiteIncerta(t *testing.T) {
	db := openIntegrationDB(t)
	f := &fakeSefaz{}
	f.authorize = func(key string) (sefaz.AuthorizeResult, error) {
		if key[34] == '1' {
			return sefaz.AuthorizeResult{}, sefaz.ErrUncertain
		}
		return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 100, "Autorizado o uso da NF-e")}, nil
	}
	f.query = func(string) (sefaz.QueryResult, error) { return sefaz.QueryResult{}, sefaz.ErrUncertain }
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)

	for i, idem := range []string{"f-1", "f-2", "f-3"} {
		if _, inv := issue(t, h, idem, issueBody(issuerID, "09012100")); inv["status"] != "uncertain" {
			t.Fatalf("nota %d: %v", i+1, inv)
		}
	}
	_, inv := issue(t, h, "f-4", issueBody(issuerID, "09012100"))
	if inv["status"] != "authorized" || inv["access_key"].(string)[34] != '6' || inv["number"].(float64) != 4 {
		t.Fatalf("quarta nota, já em contingência: %v", inv)
	}
	var uncertain int
	_ = db.QueryRow(`SELECT COUNT(*) FROM invoice_invoices WHERE status = 'uncertain' AND substr(access_key, 35, 1) = '1'`).Scan(&uncertain)
	if uncertain != 3 {
		t.Errorf("as três incertas deveriam continuar incertas, com a chave original: %d", uncertain)
	}
}

func TestContingenciaManual(t *testing.T) {
	db := openIntegrationDB(t)
	h := newServerWithSefaz(t, db, &fakeSefaz{})
	if rec, _ := do(t, h, http.MethodPost, "/v1/admin/contingency", adminToken, map[string]any{"uf": "SP", "reason": "curto"}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("motivo curto: %d", rec.Code)
	}
	if rec, _ := do(t, h, http.MethodPost, "/v1/admin/contingency", adminToken, map[string]any{"uf": "SP", "model": 65, "reason": "Parada programada da SEFAZ"}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("SVC para NFC-e: %d", rec.Code)
	}
	rec, c := do(t, h, http.MethodPost, "/v1/admin/contingency", adminToken, map[string]any{"uf": "AM", "reason": "Parada programada da SEFAZ-AM"})
	if rec.Code != http.StatusOK || c["kind"] != "SVCRS" {
		t.Errorf("AM vai para a SVC-RS: %d %v", rec.Code, c)
	}
	if rec, _ := do(t, h, http.MethodDelete, "/v1/admin/contingency/AM", adminToken, nil); rec.Code != http.StatusNoContent {
		t.Errorf("desligar: %d", rec.Code)
	}
}
