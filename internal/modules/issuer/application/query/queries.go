// Package query são as leituras do módulo.
package query

import "github.com/alfredokang/kang-fiscal/shared/pagination"

// GetIssuer lê um emitente.
type GetIssuer struct {
	TenantID string
	IssuerID string
}

// ListIssuers lista os emitentes do tenant, sempre paginado (Constituição, Art. 7).
type ListIssuers struct {
	TenantID string
	Page     pagination.Params
}
