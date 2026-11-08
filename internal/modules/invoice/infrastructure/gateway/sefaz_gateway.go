// Package gateway fala com a SEFAZ pelo pkg/sefaz.
package gateway

import (
	"context"
	"crypto/x509"
	"sync"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
)

// SefazGateway cria um cliente por chamada (cada emitente tem o seu certificado), mas
// carrega a cadeia ICP-Brasil e a tabela de endereços uma vez só.
type SefazGateway struct {
	once  sync.Once
	pool  *x509.CertPool
	table *sefaz.Table
	err   error
}

// NewSefazGateway monta o gateway.
func NewSefazGateway() *SefazGateway { return &SefazGateway{} }

func (g *SefazGateway) client(cert *certificate.Certificate) (*sefaz.Client, error) {
	g.once.Do(func() {
		if g.pool, g.err = sefaz.TrustPool(); g.err != nil {
			return
		}
		g.table, g.err = sefaz.DefaultTable()
	})
	if g.err != nil {
		return nil, g.err
	}
	return sefaz.NewClient(sefaz.Config{Certificate: cert.TLS(), Table: g.table, RootCAs: g.pool})
}

// Authorize envia a nota.
func (g *SefazGateway) Authorize(ctx context.Context, cert *certificate.Certificate, t sefaz.Target, batch int64, signed []byte) (sefaz.AuthorizeResult, error) {
	c, err := g.client(cert)
	if err != nil {
		return sefaz.AuthorizeResult{}, err
	}
	return c.Authorize(ctx, t, batch, signed)
}

// Query consulta a nota pela chave.
func (g *SefazGateway) Query(ctx context.Context, cert *certificate.Certificate, t sefaz.Target, key string) (sefaz.QueryResult, error) {
	c, err := g.client(cert)
	if err != nil {
		return sefaz.QueryResult{}, err
	}
	return c.Query(ctx, t, key)
}
