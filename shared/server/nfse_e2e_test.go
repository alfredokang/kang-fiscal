package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
	t.Helper()
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	config.Env.SecretsEncryptionKey = testKey
	srv, err := server.NewServer(db, server.WithSefaz(&fakeSefaz{}), server.WithNFSeGateway(f))
	if err != nil {
		t.Fatal(err)
	}
	return srv.Handler()
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
