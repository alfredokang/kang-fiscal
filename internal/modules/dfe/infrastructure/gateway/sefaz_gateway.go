// Package gateway fala com o Ambiente Nacional pelo pkg/sefaz.
package gateway

import (
	"context"
	"crypto/x509"
	"sync"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
)

// Gateway carrega a cadeia ICP-Brasil e a tabela uma vez e cria um cliente por chamada.
type Gateway struct {
	once  sync.Once
	pool  *x509.CertPool
	table *sefaz.Table
	err   error
}

// New monta o gateway.
func New() *Gateway { return &Gateway{} }

func (g *Gateway) client(cert *certificate.Certificate) (*sefaz.Client, error) {
	g.once.Do(func() {
		if g.pool, g.err = sefaz.TrustPool(); g.err == nil {
			g.table, g.err = sefaz.DefaultTable()
		}
	})
	if g.err != nil {
		return nil, g.err
	}
	return sefaz.NewClient(sefaz.Config{Certificate: cert.TLS(), Table: g.table, RootCAs: g.pool})
}

// Distribute consulta a distribuição de DF-e.
func (g *Gateway) Distribute(ctx context.Context, cert *certificate.Certificate, q sefaz.DistributionQuery) (sefaz.DistributionResult, error) {
	c, err := g.client(cert)
	if err != nil {
		return sefaz.DistributionResult{}, err
	}
	return c.Distribute(ctx, q)
}

// SendEvent envia a manifestação.
func (g *Gateway) SendEvent(ctx context.Context, cert *certificate.Certificate, t sefaz.Target, batch int64, signed []byte) (sefaz.EventResult, error) {
	c, err := g.client(cert)
	if err != nil {
		return sefaz.EventResult{}, err
	}
	return c.SendEvent(ctx, t, batch, signed)
}
