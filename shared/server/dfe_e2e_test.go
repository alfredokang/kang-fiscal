package server_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

const supplierKey = "35261199888777000166550010000000101000000105"

// fakeAN é o Ambiente Nacional: entrega um resumo de NF-e e um de evento na primeira
// consulta, nada nas seguintes, e o XML completo pela chave depois da ciência.
type fakeAN struct {
	calls     atomic.Int32
	manifests atomic.Int32
}

func (f *fakeAN) Distribute(_ context.Context, _ *certificate.Certificate, q sefaz.DistributionQuery) (sefaz.DistributionResult, error) {
	f.calls.Add(1)
	if q.AccessKey != "" {
		return sefaz.DistributionResult{CStat: 138, LastNSU: 2, MaxNSU: 2, Docs: []sefaz.DistributedDoc{{NSU: 1, Schema: "procNFe_v4.00.xsd",
			XML: []byte(`<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><NFe><infNFe Id="NFe` + supplierKey + `" versao="4.00"><ide><dhEmi>2026-11-07T09:00:00-03:00</dhEmi></ide><emit><CNPJ>99888777000166</CNPJ><xNome>FORNECEDOR DE GRAOS LTDA</xNome></emit><total><ICMSTot><vNF>1500.00</vNF></ICMSTot></total></infNFe></NFe></nfeProc>`)}}}, nil
	}
	if q.LastNSU >= 2 {
		return sefaz.DistributionResult{CStat: 137, Motive: "Nenhum documento localizado", LastNSU: 2, MaxNSU: 2}, nil
	}
	return sefaz.DistributionResult{CStat: 138, Motive: "Documento localizado", LastNSU: 2, MaxNSU: 2, Docs: []sefaz.DistributedDoc{
		{NSU: 1, Schema: "resNFe_v1.01.xsd", XML: []byte(`<resNFe xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.01"><chNFe>` + supplierKey + `</chNFe><CNPJ>99888777000166</CNPJ><xNome>FORNECEDOR DE GRAOS LTDA</xNome><IE>123</IE><dhEmi>2026-11-07T09:00:00-03:00</dhEmi><tpNF>1</tpNF><vNF>1500.00</vNF><digVal>x</digVal><dhRecbto>2026-11-07T09:00:01-03:00</dhRecbto><nProt>135</nProt><cSitNFe>1</cSitNFe></resNFe>`)},
		{NSU: 2, Schema: "resEvento_v1.01.xsd", XML: []byte(`<resEvento xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.01"><chNFe>` + supplierKey + `</chNFe></resEvento>`)},
	}}, nil
}

func (f *fakeAN) SendEvent(_ context.Context, _ *certificate.Certificate, t sefaz.Target, _ int64, signed []byte) (sefaz.EventResult, error) {
	f.manifests.Add(1)
	if _, err := xmldsig.Verify(signed); err != nil || t.UF != "AN" {
		return sefaz.EventResult{BatchCStat: 128, CStat: 297, Motive: "assinatura ou destino errado"}, nil
	}
	return sefaz.EventResult{BatchCStat: 128, CStat: 135, Motive: "Evento registrado", Protocol: "891260000000001"}, nil
}

func TestRecebimentoDeNotas(t *testing.T) {
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	config.Env.SecretsEncryptionKey = testKey
	an := &fakeAN{}
	srv, err := server.NewServer(db, server.WithSefaz(&fakeSefaz{}), server.WithDFeGateway(an))
	if err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	issuerID := setupEmitter(t, h)

	rec, sync := do(t, h, http.MethodPost, "/v1/dfe/sync?issuer_id="+issuerID, adminToken, nil)
	if rec.Code != http.StatusOK || sync["received"].(float64) != 2 {
		t.Fatalf("sincronização: %d %v", rec.Code, sync)
	}
	// Consultar de novo dentro da hora seria bloqueio na SEFAZ: o Kang Fiscal recusa antes.
	if rec, out := do(t, h, http.MethodPost, "/v1/dfe/sync?issuer_id="+issuerID, adminToken, nil); rec.Code != http.StatusTooManyRequests || an.calls.Load() != 1 {
		t.Errorf("segunda consulta na mesma hora: %d %v, chamadas=%d", rec.Code, out, an.calls.Load())
	}

	_, list := do(t, h, http.MethodGet, "/v1/dfe/documents?issuer_id="+issuerID, adminToken, nil)
	docs := list["data"].([]any)
	if len(docs) != 2 {
		t.Fatalf("documentos: %d", len(docs))
	}
	var summary map[string]any
	for _, d := range docs {
		if m := d.(map[string]any); m["kind"] == "summary" {
			summary = m
		}
	}
	if summary["emitter_name"] != "FORNECEDOR DE GRAOS LTDA" || summary["total"] != "1500.00" || summary["access_key"] != supplierKey {
		t.Fatalf("resumo: %v", summary)
	}

	path := "/v1/dfe/documents/" + summary["id"].(string) + "/manifest"
	if rec, _ := do(t, h, http.MethodPost, path, adminToken, map[string]any{"type": "not_performed"}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("operação não realizada sem justificativa: %d", rec.Code)
	}
	rec, man := do(t, h, http.MethodPost, path, adminToken, map[string]any{"type": "awareness"})
	if rec.Code != http.StatusOK || man["protocol"] != "891260000000001" {
		t.Fatalf("ciência: %d %v", rec.Code, man)
	}
	// Depois da ciência, o XML completo foi baixado pela chave.
	_, list = do(t, h, http.MethodGet, "/v1/dfe/documents?issuer_id="+issuerID, adminToken, nil)
	kinds := map[string]string{}
	for _, d := range list["data"].([]any) {
		m := d.(map[string]any)
		manifestation, _ := m["manifestation"].(string) // omitido quando vazio
		kinds[m["kind"].(string)] = manifestation
	}
	if _, ok := kinds["full"]; !ok || kinds["summary"] != "210210" {
		t.Errorf("depois da ciência: %v", kinds)
	}

	// O worker só sincroniza quem já pode: zera a espera e roda.
	if _, err := db.Exec(`UPDATE dfe_cursors SET next_query_at = NOW() - INTERVAL '1 second'`); err != nil {
		t.Fatal(err)
	}
	synced, received, errs := srv.DFe().SyncDue(context.Background(), 10)
	if synced != 1 || received != 0 || len(errs) != 0 {
		t.Errorf("worker: sincronizados=%d recebidos=%d erros=%v", synced, received, errs)
	}
	if synced, _, _ := srv.DFe().SyncDue(context.Background(), 10); synced != 0 {
		t.Errorf("o worker consultou de novo dentro da hora: %d", synced)
	}
}
