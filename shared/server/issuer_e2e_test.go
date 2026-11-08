package server_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

const adminToken = "token-de-teste"

// openIntegrationDB abre o Postgres DESCARTÁVEL do `make check`. Sem INTEGRATION_DB=1 o
// teste é pulado: ele nunca roda contra o banco de desenvolvimento (scripts/test-integration.sh).
func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("INTEGRATION_DB") != "1" {
		t.Skip("teste de integração: rode com make check")
	}
	db, err := sql.Open("pgx", os.Getenv("INTEGRATION_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`DELETE FROM issuer_issuers; DELETE FROM certificate_certificates`); err != nil {
		t.Fatal(err)
	}
	return db
}

func newHandler(t *testing.T, db *sql.DB) http.Handler {
	t.Helper()
	srv, err := server.NewServer(db)
	if err != nil {
		t.Fatal(err)
	}
	return srv.Handler()
}

func do(t *testing.T, h http.Handler, method, path, token string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestCicloDoEmitente(t *testing.T) {
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	h := newHandler(t, db)

	body := map[string]any{
		"cnpj":       "12.ABC.345/01DE-35",
		"legal_name": "Café Exemplo LTDA",
		"tax_regime": 3,
		"address":    map[string]any{"uf": "SP", "city_code": "3550308"},
	}

	// Sem credencial, nada.
	if rec, _ := do(t, h, http.MethodPost, "/v1/admin/issuers", "", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem token: %d", rec.Code)
	}
	if rec, _ := do(t, h, http.MethodPost, "/v1/admin/issuers", "errado", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token errado: %d", rec.Code)
	}

	rec, created := do(t, h, http.MethodPost, "/v1/admin/issuers", adminToken, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("cadastro: %d %s", rec.Code, rec.Body)
	}
	id := created["id"].(string)
	if created["environment"] != "homologation" || created["can_issue"] != false {
		t.Errorf("emitente novo: %v", created)
	}

	// O mesmo CNPJ no mesmo tenant dividiria a numeração.
	if rec, _ := do(t, h, http.MethodPost, "/v1/admin/issuers", adminToken, body); rec.Code != http.StatusConflict {
		t.Errorf("CNPJ repetido: %d", rec.Code)
	}

	// Cadastro incompleto não vai para produção.
	if rec, _ := do(t, h, http.MethodPost, "/v1/admin/issuers/"+id+"/production", adminToken, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("promoção incompleta: %d", rec.Code)
	}

	update := map[string]any{
		"legal_name": "Café Exemplo LTDA", "tax_regime": 3, "state_registration": "ISENTO",
		"address": map[string]any{
			"street": "Rua das Flores", "number": "100", "district": "Centro",
			"city_code": "3550308", "city_name": "São Paulo", "uf": "SP", "zip_code": "01001-000",
		},
	}
	rec, updated := do(t, h, http.MethodPut, "/v1/admin/issuers/"+id, adminToken, update)
	if rec.Code != http.StatusOK || updated["can_issue"] != true {
		t.Fatalf("atualização: %d %v", rec.Code, updated)
	}

	rec, promoted := do(t, h, http.MethodPost, "/v1/admin/issuers/"+id+"/production", adminToken, nil)
	if rec.Code != http.StatusOK || promoted["environment"] != "production" {
		t.Fatalf("promoção: %d %v", rec.Code, promoted)
	}

	rec, page := do(t, h, http.MethodGet, "/v1/admin/issuers?limit=500", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("lista: %d", rec.Code)
	}
	pg := page["pagination"].(map[string]any)
	if pg["limit"].(float64) != 100 || pg["total"].(float64) != 1 {
		t.Errorf("paginação: o teto é 100 (Art. 7): %v", pg)
	}

	// Outro tenant não enxerga o emitente nem pelo id.
	if _, err := db.Exec(`UPDATE issuer_issuers SET tenant_id = 'outro' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if rec, _ := do(t, h, http.MethodGet, "/v1/admin/issuers/"+id, adminToken, nil); rec.Code != http.StatusNotFound {
		t.Errorf("emitente de outro tenant: %d", rec.Code)
	}
}

func TestRotasDeAdministracaoFechadasSemToken(t *testing.T) {
	config.Env.AdminToken = ""
	h := newHandler(t, nil)
	if rec, _ := do(t, h, http.MethodGet, "/v1/admin/issuers", "qualquer", nil); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("sem ADMIN_TOKEN as rotas fecham: %d", rec.Code)
	}
	if rec, _ := do(t, h, http.MethodGet, "/health", "", nil); rec.Code != http.StatusOK {
		t.Errorf("/health: %d", rec.Code)
	}
}
