package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/alfredokang/kang-fiscal/shared/helpers"
)

// DefaultTenant é o tenant da instalação de um emitente só, enquanto as credenciais por
// tenant não existem. Quem roda o Kang Fiscal para a própria empresa nunca precisa saber
// que tenant existe.
const DefaultTenant = "default"

// AdminToken protege as rotas de administração com o ADMIN_TOKEN do ambiente.
//
// Token vazio fecha as rotas com 503 em vez de abri-las: a instalação recém-baixada do
// Docker Hub, esquecida sem configuração, não pode nascer com o cadastro de emitente
// exposto.
func AdminToken(token string) func(http.Handler) http.Handler {
	expected := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				helpers.WriteError(w, http.StatusServiceUnavailable,
					"rotas de administração desligadas: configure ADMIN_TOKEN", "ADMIN_DISABLED")
				return
			}

			got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			// Compara os hashes, e não as strings: ConstantTimeCompare devolve na hora
			// quando os tamanhos diferem, e isso vazaria o tamanho do token.
			gotSum := sha256.Sum256([]byte(got))
			if !ok || subtle.ConstantTimeCompare(gotSum[:], expected[:]) != 1 {
				helpers.WriteError(w, http.StatusUnauthorized, "credencial inválida", "UNAUTHORIZED")
				return
			}

			ctx := WithPrincipal(WithTenant(r.Context(), DefaultTenant), "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
