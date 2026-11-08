// Package middleware concentra o que toda requisição atravessa antes do handler.
package middleware

import "context"

// ctxKey é privado: outro pacote não lê nem sobrescreve estes valores com uma string de
// mesmo nome.
type ctxKey string

const (
	tenantIDKey  ctxKey = "tenant_id"
	principalKey ctxKey = "principal"
)

// TenantIDFrom devolve o tenant da requisição.
//
// Ele vem da credencial — nunca do corpo (Constituição, Art. 8): cliente que escolhe o
// próprio tenant emite nota no CNPJ do vizinho.
func TenantIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(tenantIDKey).(string)
	return v, ok && v != ""
}

// PrincipalFrom devolve quem fez a requisição ("admin" ou o id da credencial). É o que a
// trilha de auditoria registra e o que o use case confere.
func PrincipalFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(principalKey).(string)
	return v, ok && v != ""
}

// WithTenant injeta o tenant no contexto.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// WithPrincipal injeta quem fez a requisição.
func WithPrincipal(ctx context.Context, principal string) context.Context {
	return context.WithValue(ctx, principalKey, principal)
}
