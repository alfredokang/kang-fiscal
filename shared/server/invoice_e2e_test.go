package server_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

// fakeSefaz responde como a autorizadora, com o comportamento que o teste escolher.
type fakeSefaz struct {
	authorizeCalls atomic.Int32
	queryCalls     atomic.Int32
	// authorize devolve o resultado; nil = autoriza com 100.
	authorize func(key string) (sefaz.AuthorizeResult, error)
	query     func(key string) (sefaz.QueryResult, error)
	// event devolve o resultado do evento; nil = registra com 135.
	event      func(signed []byte) (sefaz.EventResult, error)
	eventCalls atomic.Int32
}

func (f *fakeSefaz) SendEvent(_ context.Context, _ *certificate.Certificate, _ sefaz.Target, _ int64, signed []byte) (sefaz.EventResult, error) {
	f.eventCalls.Add(1)
	if _, err := xmldsig.Verify(signed); err != nil {
		return sefaz.EventResult{BatchCStat: 128, CStat: 297, Motive: "Rejeição: Assinatura difere do calculado"}, nil
	}
	if f.event != nil {
		return f.event(signed)
	}
	return registered(), nil
}

func registered() sefaz.EventResult {
	return sefaz.EventResult{BatchCStat: 128, CStat: 135, Motive: "Evento registrado e vinculado a NF-e", Protocol: "135260000000777",
		RetEvento: []byte(`<retEvento xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.00"><infEvento><tpAmb>2</tpAmb><verAplic>FAKE</verAplic><cOrgao>35</cOrgao><cStat>135</cStat><xMotivo>Evento registrado e vinculado a NF-e</xMotivo><dhRegEvento>2026-11-08T11:00:00-03:00</dhRegEvento><nProt>135260000000777</nProt></infEvento></retEvento>`)}
}

func protocol(key string, cStat int, motive string) *sefaz.Protocol {
	raw := `<protNFe xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><infProt><tpAmb>2</tpAmb><verAplic>FAKE</verAplic><chNFe>` +
		key + `</chNFe><dhRecbto>2026-11-08T10:00:00-03:00</dhRecbto><nProt>135260000000001</nProt><digVal>AAAA</digVal><cStat>` +
		strconv.Itoa(cStat) + `</cStat><xMotivo>` + motive + `</xMotivo></infProt></protNFe>`
	return &sefaz.Protocol{CStat: cStat, Motive: motive, Key: key, Number: "135260000000001", Raw: []byte(raw)}
}

func keyOf(signed []byte) string {
	s := string(signed)
	i := strings.Index(s, `Id="NFe`)
	return s[i+7 : i+7+44]
}

func (f *fakeSefaz) Authorize(_ context.Context, _ *certificate.Certificate, _ sefaz.Target, _ int64, signed []byte) (sefaz.AuthorizeResult, error) {
	f.authorizeCalls.Add(1)
	if _, err := xmldsig.Verify(signed); err != nil {
		return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(keyOf(signed), 297, "Rejeição: Assinatura difere do calculado")}, nil
	}
	if f.authorize != nil {
		return f.authorize(keyOf(signed))
	}
	return sefaz.AuthorizeResult{BatchCStat: 104, BatchMotive: "Lote processado", Protocol: protocol(keyOf(signed), 100, "Autorizado o uso da NF-e")}, nil
}

func (f *fakeSefaz) Query(_ context.Context, _ *certificate.Certificate, _ sefaz.Target, key string) (sefaz.QueryResult, error) {
	f.queryCalls.Add(1)
	if f.query != nil {
		return f.query(key)
	}
	return sefaz.QueryResult{CStat: 217, Motive: "NF-e não consta na base de dados da SEFAZ"}, nil
}

// setupEmitter cadastra emitente completo, certificado e perfil, e devolve o id do emitente.
func setupEmitter(t *testing.T, h http.Handler) string {
	t.Helper()
	rec, issuer := do(t, h, http.MethodPost, "/v1/admin/issuers", adminToken, map[string]any{
		"cnpj": "11222333000181", "legal_name": "CAFE EXEMPLO LTDA", "tax_regime": 3, "state_registration": "123456789110",
		"address": map[string]any{"street": "RUA DAS FLORES", "number": "100", "district": "CENTRO",
			"city_code": "3550308", "city_name": "SAO PAULO", "uf": "SP", "zip_code": "01001000"}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("emitente: %d %s", rec.Code, rec.Body)
	}
	id := issuer["id"].(string)
	pfx, _ := certificate.TestPFX("CAFE EXEMPLO LTDA", "11222333000181", "s", time.Now().Add(365*24*time.Hour))
	if rec, _ := do(t, h, http.MethodPut, "/v1/admin/issuers/"+id+"/certificate", adminToken,
		map[string]any{"pfx_base64": base64.StdEncoding.EncodeToString(pfx), "password": "s"}); rec.Code != http.StatusOK {
		t.Fatalf("certificado: %d %s", rec.Code, rec.Body)
	}
	if rec, _ := do(t, h, http.MethodPut, "/v1/admin/tax-profiles/venda-sp", adminToken, map[string]any{
		"name": "Venda em SP", "rules": map[string]any{"cfop_internal": "5102", "cfop_interstate": "6102",
			"icms": map[string]any{"code": "00", "rate": "18"}, "pis": map[string]any{"cst": "01", "rate": "1.65"},
			"cofins": map[string]any{"cst": "01", "rate": "7.6"}, "ibs_cbs": map[string]any{"cst": "000", "class_trib": "000001"},
			"exclude_icms_from_pis_base": true}}); rec.Code != http.StatusOK {
		t.Fatalf("perfil: %d %s", rec.Code, rec.Body)
	}
	return id
}

func issueBody(issuerID, ncm string) map[string]any {
	return map[string]any{
		"issuer_id": issuerID, "nature_of_operation": "VENDA DE MERCADORIA", "final_consumer": false,
		"recipient": map[string]any{"cnpj": "99.888.777/0001-66", "name": "CLIENTE LTDA", "ie_indicator": 9,
			"address": map[string]any{"street": "AV PAULISTA", "number": "1000", "district": "BELA VISTA",
				"city_code": "3550308", "city_name": "SAO PAULO", "uf": "SP", "zip_code": "01310-100"}},
		"items": []map[string]any{{"code": "CAFE-500", "description": "CAFE TORRADO EM GRAOS 500G", "ncm": ncm,
			"unit": "UN", "quantity": "3", "unit_price": "33.33", "tax_profile": "venda-sp"}},
		"payments": []map[string]any{{"method": "17", "amount": "99.99"}},
	}
}

func issue(t *testing.T, h http.Handler, key string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return doWithHeader(t, h, http.MethodPost, "/v1/nfe", adminToken, map[string]string{"Idempotency-Key": key}, body)
}

func newServerWithSefaz(t *testing.T, db *sql.DB, f *fakeSefaz) http.Handler {
	t.Helper()
	config.Env.AdminToken = adminToken
	config.Env.SecretsEncryptionKey = testKey
	srv, err := server.NewServer(db, server.WithSefaz(f))
	if err != nil {
		t.Fatal(err)
	}
	return srv.Handler()
}

func TestEmissaoAutorizada(t *testing.T) {
	db := openIntegrationDB(t)
	f := &fakeSefaz{}
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)

	rec, inv := issue(t, h, "pedido-1001", issueBody(issuerID, "09012100"))
	if rec.Code != http.StatusCreated || inv["status"] != "authorized" || inv["number"].(float64) != 1 {
		t.Fatalf("emissão: %d %v", rec.Code, inv)
	}
	key := inv["access_key"].(string)
	if inv["total"] != "99.99" || len(key) != 44 || inv["protocol"] != "135260000000001" || inv["xml_sha256"] == "" {
		t.Errorf("nota: %v", inv)
	}

	// O XML de distribuição é o nfeProc, com a nota assinada (válida no XSD) e o protocolo.
	xrec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/nfe/"+key+"/xml", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	h.ServeHTTP(xrec, req)
	proc := xrec.Body.String()
	if !strings.HasPrefix(proc, `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><NFe`) || !strings.Contains(proc, "<protNFe") {
		t.Fatalf("nfeProc: %.200s", proc)
	}
	var signed []byte
	if err := db.QueryRow(`SELECT signed_xml FROM invoice_invoices WHERE access_key = $1`, key).Scan(&signed); err != nil {
		t.Fatal(err)
	}
	if _, err := xmldsig.Verify(signed); err != nil {
		t.Errorf("assinatura: %v", err)
	}
	if !strings.Contains(string(signed), "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL") {
		t.Error("em homologação o destinatário precisa do nome de teste (rejeição 598)")
	}
	nfetest.MustValidateXSD(t, signed)

	// A mesma Idempotency-Key devolve a mesma nota, sem enviar de novo.
	rec2, again := issue(t, h, "pedido-1001", issueBody(issuerID, "09012100"))
	if rec2.Code != http.StatusCreated || again["access_key"] != key || f.authorizeCalls.Load() != 1 {
		t.Errorf("repetição: %d %v, envios=%d", rec2.Code, again, f.authorizeCalls.Load())
	}
	// A mesma chave com outro conteúdo é conflito.
	if rec3, _ := issue(t, h, "pedido-1001", issueBody(issuerID, "21011110")); rec3.Code != http.StatusConflict {
		t.Errorf("chave reaproveitada com outro corpo: %d", rec3.Code)
	}

	// O banco recusa mexer no XML autorizado e apagar a nota — de qualquer cliente.
	if _, err := db.Exec(`UPDATE invoice_invoices SET authorized_xml = 'x' WHERE access_key = $1`, key); err == nil {
		t.Error("o XML autorizado foi alterado")
	}
	if _, err := db.Exec(`DELETE FROM invoice_invoices WHERE access_key = $1`, key); err == nil {
		t.Error("a nota autorizada foi apagada")
	}

	// O número foi consumido: a próxima nota é a 2, e a 1 não aparece como buraco.
	_, inv2 := issue(t, h, "pedido-1002", issueBody(issuerID, "09012100"))
	var open int
	_ = db.QueryRow(`SELECT COUNT(*) FROM numbering_reservations WHERE status = 'reserved'`).Scan(&open)
	if inv2["number"].(float64) != 2 || open != 0 {
		t.Errorf("segunda nota: %v, reservas abertas=%d", inv2["number"], open)
	}
}

// Conexão caída depois do envio: a nota é consultada pela chave e sai autorizada, sem
// segundo envio (Constituição, Art. 6.1).
func TestEmissaoIncertaResolvidaPorConsulta(t *testing.T) {
	db := openIntegrationDB(t)
	f := &fakeSefaz{
		authorize: func(string) (sefaz.AuthorizeResult, error) { return sefaz.AuthorizeResult{}, sefaz.ErrUncertain },
	}
	f.query = func(key string) (sefaz.QueryResult, error) {
		return sefaz.QueryResult{CStat: 100, Protocol: protocol(key, 100, "Autorizado o uso da NF-e")}, nil
	}
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)

	rec, inv := issue(t, h, "pedido-2001", issueBody(issuerID, "09012100"))
	if rec.Code != http.StatusCreated || inv["status"] != "authorized" {
		t.Fatalf("incerta: %d %v", rec.Code, inv)
	}
	if f.authorizeCalls.Load() != 1 || f.queryCalls.Load() != 1 {
		t.Errorf("envios=%d consultas=%d", f.authorizeCalls.Load(), f.queryCalls.Load())
	}
}

// Sem resposta e sem a SEFAZ conhecer a chave, a nota fica incerta — e a mesma chamada
// repetida é que reenvia o MESMO XML.
func TestEmissaoIncertaReenviadaNaRepeticao(t *testing.T) {
	db := openIntegrationDB(t)
	var fail atomic.Bool
	fail.Store(true)
	f := &fakeSefaz{}
	f.authorize = func(key string) (sefaz.AuthorizeResult, error) {
		if fail.Load() {
			return sefaz.AuthorizeResult{}, sefaz.ErrUncertain
		}
		return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 100, "Autorizado o uso da NF-e")}, nil
	}
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)

	rec, inv := issue(t, h, "pedido-3001", issueBody(issuerID, "09012100"))
	if rec.Code != http.StatusAccepted || inv["status"] != "uncertain" {
		t.Fatalf("primeira tentativa: %d %v", rec.Code, inv)
	}
	fail.Store(false)
	rec, again := issue(t, h, "pedido-3001", issueBody(issuerID, "09012100"))
	if rec.Code != http.StatusCreated || again["status"] != "authorized" || again["access_key"] != inv["access_key"] {
		t.Fatalf("repetição: %d %v", rec.Code, again)
	}
}

func TestEmissaoRejeitadaComDica(t *testing.T) {
	db := openIntegrationDB(t)
	f := &fakeSefaz{authorize: func(key string) (sefaz.AuthorizeResult, error) {
		return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 778, "Rejeição: Informado NCM inexistente")}, nil
	}}
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)
	rec, inv := issue(t, h, "pedido-4001", issueBody(issuerID, "09019999"))
	if rec.Code != http.StatusUnprocessableEntity || inv["status"] != "rejected" || inv["cstat"].(float64) != 778 || inv["hint"] == "" {
		t.Errorf("rejeição: %d %v", rec.Code, inv)
	}
}

func TestEmissaoRecusadaAntesDeReservarNumero(t *testing.T) {
	db := openIntegrationDB(t)
	h := newServerWithSefaz(t, db, &fakeSefaz{})
	issuerID := setupEmitter(t, h)
	if rec, _ := do(t, h, http.MethodDelete, "/v1/admin/issuers/"+issuerID+"/certificate", adminToken, nil); rec.Code != http.StatusNoContent {
		t.Fatal("revogar certificado")
	}
	if rec, out := issue(t, h, "pedido-5001", issueBody(issuerID, "09012100")); rec.Code != http.StatusUnprocessableEntity || out["code"] != "NO_CERTIFICATE" {
		t.Errorf("sem certificado: %d %v", rec.Code, out)
	}
	if rec, out := issue(t, h, "", issueBody(issuerID, "09012100")); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("sem Idempotency-Key: %d %v", rec.Code, out)
	}
	var reservations int
	_ = db.QueryRow(`SELECT COUNT(*) FROM numbering_reservations`).Scan(&reservations)
	if reservations != 0 {
		t.Errorf("número reservado sem a nota poder sair: %d", reservations)
	}
}
