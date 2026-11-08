package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/alfredokang/kang-fiscal/shared/config"
)

func createKey(t *testing.T, h http.Handler, tenant string, scopes ...string) (string, string) {
	t.Helper()
	rec, out := do(t, h, http.MethodPost, "/v1/admin/credentials", adminToken,
		map[string]any{"tenant_id": tenant, "name": "ERP " + tenant, "scopes": scopes})
	if rec.Code != http.StatusCreated {
		t.Fatalf("credencial: %d %v", rec.Code, out)
	}
	return out["id"].(string), out["token"].(string)
}

func TestCredenciaisPorTenant(t *testing.T) {
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	config.Env.CredentialPepper = "pepper-de-teste"
	config.Env.RateLimitPerMinute = 0
	h := newHandler(t, db)

	_, tokenA := createKey(t, h, "loja-a", "setup", "nfe:read", "nfe:write")
	idB, tokenB := createKey(t, h, "loja-b", "nfe:read")
	if !strings.HasPrefix(tokenA, "kf_") {
		t.Fatalf("token: %s", tokenA)
	}

	// O segredo não está no banco: só o HMAC.
	var stored string
	_ = db.QueryRow(`SELECT hash FROM credential_keys WHERE tenant_id = 'loja-a'`).Scan(&stored)
	if strings.Contains(tokenA, stored) || len(stored) != 64 {
		t.Errorf("hash guardado: %q", stored)
	}
	// A listagem nunca devolve o token.
	_, list := do(t, h, http.MethodGet, "/v1/admin/credentials?tenant_id=loja-a", adminToken, nil)
	if first := list["data"].([]any)[0].(map[string]any); first["token"] != nil {
		t.Error("a listagem devolveu o token")
	}

	issuer := map[string]any{"cnpj": "11222333000181", "legal_name": "LOJA A LTDA", "tax_regime": 1, "address": map[string]any{"uf": "SP"}}
	rec, created := do(t, h, http.MethodPost, "/v1/issuers", tokenA, issuer)
	if rec.Code != http.StatusCreated {
		t.Fatalf("emitente da loja A: %d %v", rec.Code, created)
	}
	id := created["id"].(string)

	// Isolamento: a loja B não enxerga o emitente da A, nem pelo id.
	if rec, _ := do(t, h, http.MethodGet, "/v1/issuers/"+id, tokenB, nil); rec.Code != http.StatusForbidden {
		// nfe:read não alcança /issuers (setup): 403 antes mesmo do isolamento.
		t.Errorf("loja B lendo emitente: %d", rec.Code)
	}
	_, tokenB2 := createKey(t, h, "loja-b", "setup")
	if rec, _ := do(t, h, http.MethodGet, "/v1/issuers/"+id, tokenB2, nil); rec.Code != http.StatusNotFound {
		t.Errorf("loja B com setup lendo emitente da A: %d", rec.Code)
	}
	// O ADMIN_TOKEN fala pelo tenant default: também não vê a loja A.
	if rec, _ := do(t, h, http.MethodGet, "/v1/admin/issuers/"+id, adminToken, nil); rec.Code != http.StatusNotFound {
		t.Errorf("tenant default lendo emitente da A: %d", rec.Code)
	}

	// Escopo: só leitura não cria emitente nem emite nota.
	if rec, _ := do(t, h, http.MethodPost, "/v1/issuers", tokenB, issuer); rec.Code != http.StatusForbidden {
		t.Errorf("nfe:read criando emitente: %d", rec.Code)
	}
	if rec, _ := do(t, h, http.MethodPost, "/v1/nfe", tokenB, map[string]any{}); rec.Code != http.StatusForbidden {
		t.Errorf("nfe:read emitindo: %d", rec.Code)
	}
	if rec, out := do(t, h, http.MethodGet, "/v1/nfe", tokenB, nil); rec.Code != http.StatusOK || len(out["data"].([]any)) != 0 {
		t.Errorf("nfe:read listando: %d %v", rec.Code, out)
	}

	// A credencial de tenant não entra na administração.
	if rec, _ := do(t, h, http.MethodPost, "/v1/admin/credentials", tokenA, map[string]any{"tenant_id": "x", "name": "x", "scopes": []string{"setup"}}); rec.Code != http.StatusUnauthorized {
		t.Errorf("tenant criando credencial: %d", rec.Code)
	}
	if rec, _ := do(t, h, http.MethodPost, "/v1/admin/tax-rates", tokenA, map[string]any{}); rec.Code != http.StatusUnauthorized {
		t.Errorf("tenant cadastrando alíquota: %d", rec.Code)
	}

	// Token errado, adulterado e revogado: o mesmo 401.
	for _, bad := range []string{"kf_naoexiste_" + strings.Repeat("x", 40), tokenB[:len(tokenB)-1] + "x", "qualquer"} {
		if rec, _ := do(t, h, http.MethodGet, "/v1/nfe", bad, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q: %d", bad[:12], rec.Code)
		}
	}
	if rec, _ := do(t, h, http.MethodDelete, "/v1/admin/credentials/"+idB, adminToken, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("revogação: %d", rec.Code)
	}
	if rec, _ := do(t, h, http.MethodGet, "/v1/nfe", tokenB, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("credencial revogada: %d", rec.Code)
	}
}

func TestLimiteDeRequisicoes(t *testing.T) {
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	config.Env.CredentialPepper = "pepper-de-teste"
	config.Env.RateLimitPerMinute = 5
	defer func() { config.Env.RateLimitPerMinute = 0 }()
	h := newHandler(t, db)
	_, token := createKey(t, h, "loja-c", "nfe:read")
	for i := 0; i < 5; i++ {
		if rec, _ := do(t, h, http.MethodGet, "/v1/nfe", token, nil); rec.Code != http.StatusOK {
			t.Fatalf("requisição %d: %d", i+1, rec.Code)
		}
	}
	rec, _ := do(t, h, http.MethodGet, "/v1/nfe", token, nil)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("sexta requisição: %d", rec.Code)
	}
}

func TestSemPepperCredencialRecusada(t *testing.T) {
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	config.Env.CredentialPepper = ""
	h := newHandler(t, db)
	rec, _ := do(t, h, http.MethodPost, "/v1/admin/credentials", adminToken, map[string]any{"tenant_id": "loja-d", "name": "x", "scopes": []string{"setup"}})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("sem pepper: %d", rec.Code)
	}
}
