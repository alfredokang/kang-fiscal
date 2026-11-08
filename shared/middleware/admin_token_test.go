package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alfredokang/kang-fiscal/shared/middleware"
)

func TestAdminToken(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, _ := middleware.TenantIDFrom(r.Context())
		principal, _ := middleware.PrincipalFrom(r.Context())
		if tenant != middleware.DefaultTenant || principal != "admin" {
			t.Errorf("contexto: tenant=%q principal=%q", tenant, principal)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	cases := []struct {
		name   string
		token  string
		header string
		want   int
	}{
		// Sem token configurado as rotas fecham — nunca abrem.
		{"sem ADMIN_TOKEN fecha", "", "Bearer qualquer", http.StatusServiceUnavailable},
		{"sem cabeçalho", "segredo", "", http.StatusUnauthorized},
		{"token errado", "segredo", "Bearer outro", http.StatusUnauthorized},
		{"sem o prefixo Bearer", "segredo", "segredo", http.StatusUnauthorized},
		{"token certo", "segredo", "Bearer segredo", http.StatusNoContent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			rec := httptest.NewRecorder()
			middleware.AdminToken(c.token)(ok).ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Errorf("status: esperava %d, veio %d", c.want, rec.Code)
			}
		})
	}
}
