package server_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfse"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

const nfseKey = "35503082211222333000181000000000000126110000000017"

// fakeSefin é a Sefin Nacional. timeout faz o primeiro envio "sumir" depois de a nota
// ter sido gerada — o caso em que reenviar daria duas NFS-e para o mesmo serviço.
type fakeSefin struct {
	timeout   atomic.Bool
	issued    atomic.Int32
	generated atomic.Value // Id da DPS que virou nota

	mu          sync.Mutex
	events      map[string]bool // tipos registrados na nota
	eventSends  int
	eventLost   bool // o próximo evento é registrado e a resposta se perde
	linkedFirst bool // o próximo envio volta E0840
}

func (f *fakeSefin) SendEvent(_ context.Context, _ *certificate.Certificate, _ int, key string, signed []byte) (nfse.EventResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventSends++
	if _, err := xmldsig.Verify(signed); err != nil || key != nfseKey {
		return nfse.EventResult{Errors: []nfse.Message{{Code: "E0001", Description: "assinatura ou chave"}}}, nil
	}
	kind := nfse.EventCancel
	if strings.Contains(string(signed), "<e101103>") {
		kind = nfse.EventCancelAnalysis
	}
	if f.linkedFirst || f.events[nfse.EventCancel] || (kind == nfse.EventCancel && f.events[nfse.EventCancelAnalysis]) {
		f.linkedFirst = false
		return nfse.EventResult{HTTPStatus: 400, Errors: []nfse.Message{{Code: nfse.ErrAlreadyLinked, Description: "já há evento vinculado"}}}, nil
	}
	f.events[kind] = true
	if f.eventLost {
		f.eventLost = false
		return nfse.EventResult{}, nfse.ErrUncertain
	}
	return nfse.EventResult{HTTPStatus: 201, EventXML: []byte(`<evento xmlns="http://www.sped.fazenda.gov.br/nfse"/>`)}, nil
}

func (f *fakeSefin) EventRegistered(_ context.Context, _ *certificate.Certificate, _ int, _, kind string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events[kind], nil
}

func (f *fakeSefin) CancelState(_ context.Context, _ *certificate.Certificate, _ int, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.events[nfse.EventCancel] || f.events[nfse.EventAnalysisGranted]:
		return nfse.CancelStateCanceled, nil
	case f.events[nfse.EventCancelAnalysis] && f.events[nfse.EventAnalysisDenied]:
		return nfse.CancelStateDenied, nil
	case f.events[nfse.EventCancelAnalysis]:
		return nfse.CancelStateAnalysis, nil
	}
	return nfse.CancelStateNone, nil
}

func (f *fakeSefin) Issue(_ context.Context, _ *certificate.Certificate, _ int, signed []byte) (nfse.IssueResult, error) {
	f.issued.Add(1)
	if _, err := xmldsig.Verify(signed); err != nil {
		return nfse.IssueResult{HTTPStatus: 400, Errors: []nfse.Message{{Code: "E0714", Description: "assinatura inválida"}}}, nil
	}
	if strings.Contains(string(signed), "<xDescServ>RECUSAR") {
		return nfse.IssueResult{HTTPStatus: 400, Errors: []nfse.Message{{Code: "E0310", Description: "código de tributação inexistente"}}}, nil
	}
	i := strings.Index(string(signed), `Id="DPS`)
	f.generated.Store(string(signed)[i+4 : i+4+45])
	if f.timeout.Load() {
		return nfse.IssueResult{}, nfse.ErrUncertain
	}
	return nfse.IssueResult{HTTPStatus: 201, AccessKey: nfseKey, Number: "17", NFSeXML: nfseDoc()}, nil
}

func (f *fakeSefin) DPSKey(_ context.Context, _ *certificate.Certificate, _ int, id string) (string, bool, error) {
	if g, _ := f.generated.Load().(string); g == id {
		return nfseKey, true, nil
	}
	return "", false, nil
}

func (f *fakeSefin) Get(context.Context, *certificate.Certificate, int, string) ([]byte, bool, error) {
	return nfseDoc(), true, nil
}

func nfseDoc() []byte {
	return []byte(`<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.01"><infNFSe Id="NFS` + nfseKey + `"><nNFSe>17</nNFSe></infNFSe></NFSe>`)
}

func nfseBody(issuerID, description string) map[string]any {
	return map[string]any{"issuer_id": issuerID, "series": 1, "competence": "2026-11-07",
		"service": map[string]any{"national_code": "01.07.01", "description": description},
		"amount":  "1500.00", "iss": map[string]any{"taxation": 1, "retention": 1, "rate": "2"},
		"recipient": map[string]any{"cnpj": "99.888.777/0001-66", "name": "CLIENTE LTDA", "city_code": "3550308",
			"zip_code": "01310-100", "street": "AV PAULISTA", "number": "1000", "district": "BELA VISTA"},
		"ibs_cbs": map[string]any{"cst": "000", "class_trib": "000001", "operation_code": "100301"}}
}

func newServerWithSefin(t *testing.T, f *fakeSefin) http.Handler {
	h, _ := newSefinServer(t, f)
	return h
}

func newSefinServer(t *testing.T, f *fakeSefin) (http.Handler, *server.Server) {
	h, srv, _ := newSefinServerDB(t, f)
	return h, srv
}

func newSefinServerDB(t *testing.T, f *fakeSefin) (http.Handler, *server.Server, *sql.DB) {
	t.Helper()
	if f.events == nil {
		f.events = map[string]bool{}
	}
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	config.Env.SecretsEncryptionKey = testKey
	srv, err := server.NewServer(db, server.WithSefaz(&fakeSefaz{}), server.WithNFSeGateway(f))
	if err != nil {
		t.Fatal(err)
	}
	return srv.Handler(), srv, db
}

func emitNFSe(t *testing.T, h http.Handler, key string, body any) (int, map[string]any) {
	t.Helper()
	rec, out := doWithHeader(t, h, http.MethodPost, "/v1/nfse", adminToken, map[string]string{"Idempotency-Key": key}, body)
	return rec.Code, out
}

func TestNFSeNacional(t *testing.T) {
	f := &fakeSefin{}
	h := newServerWithSefin(t, f)
	issuerID := setupEmitter(t, h)

	code, inv := emitNFSe(t, h, "nfse-1", nfseBody(issuerID, "CONSULTORIA EM SISTEMAS"))
	if code != http.StatusCreated || inv["status"] != "authorized" || inv["access_key"] != nfseKey || inv["nfse_number"] != "17" {
		t.Fatalf("emissão: %d %v", code, inv)
	}
	if id := inv["dps_id"].(string); !strings.HasPrefix(id, "DPS3550308211222333000181") || len(id) != 45 {
		t.Errorf("Id da DPS: %s", id)
	}
	// Repetir a requisição devolve a mesma nota, sem novo envio.
	if code, again := emitNFSe(t, h, "nfse-1", nfseBody(issuerID, "CONSULTORIA EM SISTEMAS")); code != http.StatusCreated || again["id"] != inv["id"] || f.issued.Load() != 1 {
		t.Errorf("repetição: %d %v, envios=%d", code, again, f.issued.Load())
	}
	if code, _ := emitNFSe(t, h, "nfse-1", nfseBody(issuerID, "OUTRO SERVICO")); code != http.StatusConflict {
		t.Errorf("mesma chave, corpo diferente: %d", code)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/nfse/"+nfseKey+"/xml", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<nNFSe>17</nNFSe>") {
		t.Errorf("XML: %d", rec.Code)
	}

	code, rej := emitNFSe(t, h, "nfse-2", nfseBody(issuerID, "RECUSAR ESTE SERVICO"))
	if code != http.StatusUnprocessableEntity || rej["status"] != "rejected" || rej["error_code"] != "E0310" {
		t.Errorf("rejeição: %d %v", code, rej)
	}
	if code, _ := emitNFSe(t, h, "nfse-3", map[string]any{"issuer_id": issuerID, "service": map[string]any{"national_code": "1"}}); code != http.StatusUnprocessableEntity {
		t.Errorf("rascunho inválido: %d", code)
	}

	_, list := do(t, h, http.MethodGet, "/v1/nfse", adminToken, nil)
	if n := len(list["data"].([]any)); n != 2 {
		t.Errorf("lista: %d", n)
	}
}

// A Sefin gerou a nota, mas a resposta se perdeu: o Kang Fiscal consulta pelo Id da DPS e
// recupera a nota, em vez de reenviar e gerar uma segunda NFS-e.
func TestNFSeTimeoutNaoDuplica(t *testing.T) {
	f := &fakeSefin{}
	f.timeout.Store(true)
	h := newServerWithSefin(t, f)
	issuerID := setupEmitter(t, h)

	code, inv := emitNFSe(t, h, "nfse-t", nfseBody(issuerID, "CONSULTORIA EM SISTEMAS"))
	if code != http.StatusCreated || inv["status"] != "authorized" || inv["access_key"] != nfseKey {
		t.Fatalf("depois do timeout: %d %v", code, inv)
	}
	if f.issued.Load() != 1 {
		t.Errorf("a DPS foi reenviada: %d envios", f.issued.Load())
	}
}

func cancelNFSe(t *testing.T, h http.Handler, body map[string]any) (int, map[string]any) {
	t.Helper()
	rec, out := do(t, h, http.MethodPost, "/v1/nfse/"+nfseKey+"/cancel", adminToken, body)
	return rec.Code, out
}

func nfseStatus(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	_, out := do(t, h, http.MethodGet, "/v1/nfse/"+nfseKey, adminToken, nil)
	return out
}

func TestCancelamentoDaNFSe(t *testing.T) {
	f := &fakeSefin{}
	h, _, db := newSefinServerDB(t, f)
	issuerID := setupEmitter(t, h)
	if code, _ := emitNFSe(t, h, "c-1", nfseBody(issuerID, "CONSULTORIA EM SISTEMAS")); code != http.StatusCreated {
		t.Fatalf("emissão: %d", code)
	}
	if code, _ := cancelNFSe(t, h, map[string]any{"reason": 1, "justification": "curta"}); code != http.StatusUnprocessableEntity {
		t.Errorf("justificativa curta: %d", code)
	}
	code, ev := cancelNFSe(t, h, map[string]any{"reason": 1, "justification": "Valor do servico informado errado"})
	if code != http.StatusOK || ev["status"] != "registered" || ev["type"] != "101101" {
		t.Fatalf("cancelamento: %d %v", code, ev)
	}
	if st := nfseStatus(t, h); st["status"] != "canceled" {
		t.Errorf("nota depois do cancelamento: %v", st)
	}
	// Repetir devolve o registrado, sem novo envio.
	if code, again := cancelNFSe(t, h, map[string]any{"reason": 1, "justification": "Valor do servico informado errado"}); code != http.StatusOK || again["id"] != ev["id"] || f.eventSends != 1 {
		t.Errorf("repetição: %d %v, envios=%d", code, again, f.eventSends)
	}
	_, list := do(t, h, http.MethodGet, "/v1/nfse/"+nfseKey+"/events", adminToken, nil)
	if n := len(list["data"].([]any)); n != 1 {
		t.Errorf("eventos: %d", n)
	}
	// O banco é a última trava: a nota cancelada e o evento registrado não mudam nem somem.
	for _, q := range []string{
		`UPDATE nfse_documents SET status = 'authorized' WHERE access_key = '` + nfseKey + `'`,
		`DELETE FROM nfse_documents WHERE access_key = '` + nfseKey + `'`,
		`UPDATE nfse_events SET status = 'rejected'`,
		`DELETE FROM nfse_events`,
	} {
		if _, err := db.Exec(q); err == nil {
			t.Errorf("o gatilho deixou passar: %s", q)
		}
	}
}

// O E0840 diz só que há ALGUM evento de cancelamento: aqui é um pedido de análise fiscal
// pendente, e a nota NÃO pode ser marcada como cancelada.
func TestE0840ComAnalisePendenteNaoCancela(t *testing.T) {
	f := &fakeSefin{}
	h, srv := newSefinServer(t, f)
	issuerID := setupEmitter(t, h)
	emitNFSe(t, h, "a-1", nfseBody(issuerID, "CONSULTORIA EM SISTEMAS"))

	code, ev := cancelNFSe(t, h, map[string]any{"reason": 2, "justification": "O servico nao chegou a ser prestado", "mode": "analysis"})
	if code != http.StatusOK || ev["type"] != "101103" || ev["status"] != "registered" {
		t.Fatalf("pedido de análise: %d %v", code, ev)
	}
	if st := nfseStatus(t, h); st["status"] != "authorized" || st["cancel_state"] != "analysis" {
		t.Errorf("a nota segue válida em análise: %v", st)
	}
	code, direct := cancelNFSe(t, h, map[string]any{"reason": 2, "justification": "O servico nao chegou a ser prestado"})
	if code != http.StatusUnprocessableEntity || direct["error_code"] != "E0840" {
		t.Errorf("cancelamento direto com análise pendente: %d %v", code, direct)
	}
	if st := nfseStatus(t, h); st["status"] != "authorized" {
		t.Fatalf("o E0840 marcou como cancelada uma nota válida: %v", st)
	}
	// O município deferiu: o worker vê o evento na nota e a marca como cancelada.
	f.mu.Lock()
	f.events[nfse.EventAnalysisGranted] = true
	f.mu.Unlock()
	if res := srv.NFSe().Sweep(context.Background(), 0, 10); res.Analyses != 1 || len(res.Errors) != 0 {
		t.Errorf("worker: %+v", res)
	}
	if st := nfseStatus(t, h); st["status"] != "canceled" {
		t.Errorf("depois do deferimento: %v", st)
	}
}

// O cancelamento foi registrado e a resposta se perdeu: o pedido fica incerto, e o worker
// confere na Sefin em vez de reenviar às cegas.
func TestCancelamentoIncertoConferidoPeloWorker(t *testing.T) {
	f := &fakeSefin{}
	h, srv := newSefinServer(t, f)
	issuerID := setupEmitter(t, h)
	emitNFSe(t, h, "u-1", nfseBody(issuerID, "CONSULTORIA EM SISTEMAS"))

	f.eventLost = true
	code, ev := cancelNFSe(t, h, map[string]any{"reason": 9, "justification": "Cliente desistiu do contrato"})
	if code != http.StatusAccepted || ev["status"] != "uncertain" {
		t.Fatalf("sem resposta: %d %v", code, ev)
	}
	if st := nfseStatus(t, h); st["status"] != "authorized" {
		t.Errorf("incerto não é cancelado: %v", st)
	}
	if res := srv.NFSe().Sweep(context.Background(), 0, 10); res.Events != 1 || len(res.Errors) != 0 {
		t.Errorf("worker: %+v", res)
	}
	if st := nfseStatus(t, h); st["status"] != "canceled" || f.eventSends != 1 {
		t.Errorf("depois da conferência: %v, envios=%d", st, f.eventSends)
	}
}
