package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alfredokang/kang-fiscal/shared/helpers"
)

// Principal é quem fez a requisição.
type Principal struct {
	ID       string // "admin" ou o id da credencial
	TenantID string
	Scopes   []string
	Admin    bool // o ADMIN_TOKEN tem todos os escopos no tenant default
}

// Resolver confere um token de credencial (kf_...). Vem do composition root: o middleware
// não conhece o módulo de credenciais.
type Resolver func(ctx context.Context, token string) (Principal, error)

const scopesKey ctxKey = "scopes"

// ScopesFrom devolve os escopos da requisição.
func ScopesFrom(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(scopesKey).([]string)
	return v, ok
}

// Authenticate aceita o ADMIN_TOKEN (tenant default, todos os escopos) ou uma credencial
// do tenant. O tenant vem SEMPRE da credencial, nunca do corpo (Constituição, Art. 8).
func Authenticate(adminToken string, resolve Resolver) func(http.Handler) http.Handler {
	adminSum := sha256.Sum256([]byte(adminToken))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				helpers.WriteError(w, http.StatusUnauthorized, "credencial ausente", "UNAUTHORIZED")
				return
			}
			var p Principal
			gotSum := sha256.Sum256([]byte(token))
			switch {
			case adminToken != "" && subtle.ConstantTimeCompare(gotSum[:], adminSum[:]) == 1:
				p = Principal{ID: "admin", TenantID: DefaultTenant, Admin: true}
			case strings.HasPrefix(token, "kf_") && resolve != nil:
				var err error
				if p, err = resolve(r.Context(), token); err != nil {
					helpers.WriteError(w, http.StatusUnauthorized, "credencial inválida", "UNAUTHORIZED")
					return
				}
			default:
				helpers.WriteError(w, http.StatusUnauthorized, "credencial inválida", "UNAUTHORIZED")
				return
			}
			ctx := WithPrincipal(WithTenant(r.Context(), p.TenantID), p.ID)
			scopes := p.Scopes
			if p.Admin {
				scopes = []string{"*"}
			}
			ctx = context.WithValue(ctx, scopesKey, scopes)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope confere o escopo que a regra exige para a rota. Regra que devolve vazio é
// rota sem escopo definido — negada: rota nova esquecida na tabela não vira acesso livre.
func RequireScope(rule func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			need := rule(r)
			scopes, _ := ScopesFrom(r.Context())
			for _, s := range scopes {
				if need != "" && (s == "*" || s == need) {
					next.ServeHTTP(w, r)
					return
				}
			}
			helpers.WriteError(w, http.StatusForbidden, "a credencial não tem o escopo "+need, "FORBIDDEN")
		})
	}
}

// RateLimit limita as requisições por credencial numa janela de um minuto, em memória.
//
// Em memória e por réplica, de propósito: o Kang Fiscal só exige Postgres (Constituição,
// Art. 0), e o limite existe para conter abuso e laço de integração, não para cobrar uso.
func RateLimit(perMinute int) func(http.Handler) http.Handler {
	if perMinute <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	type window struct {
		start time.Time
		count int
	}
	var mu sync.Mutex
	windows := map[string]*window{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, _ := PrincipalFrom(r.Context())
			now := time.Now()
			mu.Lock()
			win := windows[id]
			if win == nil || now.Sub(win.start) >= time.Minute {
				win = &window{start: now}
				windows[id] = win
				if len(windows) > 10_000 { // teto de memória: zera as janelas velhas
					for k, v := range windows {
						if now.Sub(v.start) >= time.Minute {
							delete(windows, k)
						}
					}
				}
			}
			win.count++
			over := win.count > perMinute
			retry := int(time.Minute.Seconds() - now.Sub(win.start).Seconds())
			mu.Unlock()
			if over {
				w.Header().Set("Retry-After", strconv.Itoa(max(retry, 1)))
				helpers.WriteError(w, http.StatusTooManyRequests, "limite de requisições por minuto atingido", "RATE_LIMITED")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
