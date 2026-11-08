package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
)

func authorizedInvoice(t *testing.T, h http.Handler, idem string) string {
	t.Helper()
	issuerID := setupEmitter(t, h)
	rec, inv := issue(t, h, idem, issueBody(issuerID, "09012100"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("emissão: %d %v", rec.Code, inv)
	}
	return inv["access_key"].(string)
}

func TestCancelamento(t *testing.T) {
	db := openIntegrationDB(t)
	f := &fakeSefaz{}
	h := newServerWithSefaz(t, db, f)
	key := authorizedInvoice(t, h, "pedido-c1")
	path := "/v1/nfe/" + key

	if rec, _ := do(t, h, http.MethodPost, path+"/cancel", adminToken, map[string]any{"justification": "curta"}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("justificativa curta: %d", rec.Code)
	}
	rec, ev := do(t, h, http.MethodPost, path+"/cancel", adminToken, map[string]any{"justification": "Cliente desistiu da compra antes da entrega"})
	if rec.Code != http.StatusOK || ev["status"] != "registered" || ev["protocol"] != "135260000000777" || ev["type"] != "cancellation" {
		t.Fatalf("cancelamento: %d %v", rec.Code, ev)
	}
	if _, inv := do(t, h, http.MethodGet, path, adminToken, nil); inv["status"] != "canceled" {
		t.Errorf("a nota não ficou cancelada: %v", inv["status"])
	}

	// Pedir de novo devolve o mesmo cancelamento, sem enviar.
	_, again := do(t, h, http.MethodPost, path+"/cancel", adminToken, map[string]any{"justification": "Outra justificativa qualquer aqui"})
	if again["id"] != ev["id"] || f.eventCalls.Load() != 1 {
		t.Errorf("repetição: %v, envios=%d", again, f.eventCalls.Load())
	}
	// Carta de correção em nota cancelada: não.
	if rec, _ := do(t, h, http.MethodPost, path+"/correction", adminToken, map[string]any{"text": "Corrigir o complemento do endereco"}); rec.Code != http.StatusConflict {
		t.Errorf("CC-e em nota cancelada: %d", rec.Code)
	}

	// O procEventoNFe é o evento assinado (válido no XSD do lote) com o retorno.
	xrec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/nfe/events/"+ev["id"].(string)+"/xml", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	h.ServeHTTP(xrec, req)
	proc := xrec.Body.String()
	if !strings.HasPrefix(proc, `<procEventoNFe xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.00"><evento`) || !strings.Contains(proc, "<retEvento") {
		t.Fatalf("procEventoNFe: %.200s", proc)
	}
	var signed []byte
	_ = db.QueryRow(`SELECT signed_xml FROM invoice_events WHERE id = $1`, ev["id"]).Scan(&signed)
	nfetest.MustValidateSchema(t, []byte(`<envEvento xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.00"><idLote>1</idLote>`+string(signed)+`</envEvento>`), "envEventoCancNFe_v1.00.xsd")

	// O evento registrado é imutável, e a nota cancelada não volta a autorizada.
	if _, err := db.Exec(`UPDATE invoice_events SET text = 'x' WHERE id = $1`, ev["id"]); err == nil {
		t.Error("evento registrado alterado")
	}
	if _, err := db.Exec(`UPDATE invoice_invoices SET status = 'authorized' WHERE access_key = $1`, key); err == nil {
		t.Error("nota cancelada voltou a autorizada")
	}
}

// Cancelamento sem resposta e repetido: se o primeiro chegou, a SEFAZ diz duplicidade (573),
// e isso é registro, não erro.
func TestCancelamentoIncertoERepetido(t *testing.T) {
	db := openIntegrationDB(t)
	var calls atomic.Int32
	f := &fakeSefaz{event: func([]byte) (sefaz.EventResult, error) {
		if calls.Add(1) == 1 {
			return sefaz.EventResult{}, sefaz.ErrUncertain
		}
		return sefaz.EventResult{BatchCStat: 128, CStat: 573, Motive: "Rejeição: Duplicidade de evento"}, nil
	}}
	h := newServerWithSefaz(t, db, f)
	key := authorizedInvoice(t, h, "pedido-c2")
	body := map[string]any{"justification": "Cliente desistiu da compra antes da entrega"}
	if rec, ev := do(t, h, http.MethodPost, "/v1/nfe/"+key+"/cancel", adminToken, body); rec.Code != http.StatusAccepted || ev["status"] != "uncertain" {
		t.Fatalf("primeiro envio: %d %v", rec.Code, ev)
	}
	rec, ev := do(t, h, http.MethodPost, "/v1/nfe/"+key+"/cancel", adminToken, body)
	if rec.Code != http.StatusOK || ev["status"] != "registered" {
		t.Fatalf("repetição: %d %v", rec.Code, ev)
	}
	if _, inv := do(t, h, http.MethodGet, "/v1/nfe/"+key, adminToken, nil); inv["status"] != "canceled" {
		t.Errorf("nota: %v", inv["status"])
	}
}

func TestCartaDeCorrecao(t *testing.T) {
	db := openIntegrationDB(t)
	var reject atomic.Bool
	f := &fakeSefaz{}
	f.event = func([]byte) (sefaz.EventResult, error) {
		if reject.Load() {
			return sefaz.EventResult{BatchCStat: 128, CStat: 491, Motive: "Rejeição: O tpEvento informado inválido"}, nil
		}
		return registered(), nil
	}
	h := newServerWithSefaz(t, db, f)
	key := authorizedInvoice(t, h, "pedido-cc")
	path := "/v1/nfe/" + key + "/correction"

	_, first := do(t, h, http.MethodPost, path, adminToken, map[string]any{"text": "Complemento do endereco: SALA 2"})
	if first["status"] != "registered" || first["seq"].(float64) != 1 {
		t.Fatalf("primeira CC-e: %v", first)
	}
	// Rejeitada não consome a sequência: a próxima tentativa usa a 2 de novo.
	reject.Store(true)
	if rec, ev := do(t, h, http.MethodPost, path, adminToken, map[string]any{"text": "Complemento: SALA 2; transportadora: X"}); rec.Code != http.StatusUnprocessableEntity || ev["seq"].(float64) != 2 {
		t.Fatalf("CC-e rejeitada: %d %v", rec.Code, ev)
	}
	reject.Store(false)
	if _, ev := do(t, h, http.MethodPost, path, adminToken, map[string]any{"text": "Complemento: SALA 2; transportadora: X"}); ev["status"] != "registered" || ev["seq"].(float64) != 2 {
		t.Fatalf("segunda CC-e: %v", ev)
	}
	_, list := do(t, h, http.MethodGet, "/v1/nfe/"+key+"/events", adminToken, nil)
	if n := len(list["data"].([]any)); n != 3 {
		t.Errorf("eventos: %d", n)
	}
}
