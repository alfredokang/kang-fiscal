package server

import (
	"context"
	"net/http"
	"strings"

	credusecase "github.com/alfredokang/kang-fiscal/internal/modules/credential/application/usecase"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
)

// scopeFor é a tabela única de escopos das rotas de tenant. Rota que não aparece aqui
// devolve "" e é negada (middleware.RequireScope): esquecer de classificar uma rota nova
// fecha a rota, não abre.
func scopeFor(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/v1")
	read := r.Method == http.MethodGet
	switch {
	case strings.HasPrefix(path, "/nfe"), strings.HasPrefix(path, "/nfse"), strings.HasPrefix(path, "/dfe"):
		if read {
			return "nfe:read"
		}
		return "nfe:write"
	case path == "/tax/simulate", path == "/tax-rates" && read:
		return "nfe:read"
	case strings.HasPrefix(path, "/issuers"), strings.HasPrefix(path, "/tax-profiles"):
		return "setup"
	}
	return ""
}

// credentialResolver liga o middleware ao módulo de credenciais.
func credentialResolver(svc *credusecase.CredentialService) middleware.Resolver {
	return func(ctx context.Context, token string) (middleware.Principal, error) {
		k, err := svc.Resolve(ctx, token)
		if err != nil {
			return middleware.Principal{}, err
		}
		return middleware.Principal{ID: k.ID, TenantID: k.TenantID, Scopes: k.Scopes}, nil
	}
}
