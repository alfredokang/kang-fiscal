// Package gateway fala com a Sefin Nacional pelo pkg/nfse.
package gateway

import (
	"context"
	"crypto/x509"
	"sync"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfse"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
)

// Gateway carrega a cadeia ICP-Brasil uma vez e cria um cliente por chamada.
type Gateway struct {
	once sync.Once
	pool *x509.CertPool
	err  error
}

// New monta o gateway.
func New() *Gateway { return &Gateway{} }

func (g *Gateway) client(cert *certificate.Certificate, env int) (*nfse.Client, error) {
	g.once.Do(func() { g.pool, g.err = sefaz.TrustPool() })
	if g.err != nil {
		return nil, g.err
	}
	return nfse.NewClient(nfse.Config{Certificate: cert.TLS(), Environment: env, RootCAs: g.pool})
}

// Issue envia a DPS.
func (g *Gateway) Issue(ctx context.Context, cert *certificate.Certificate, env int, signed []byte) (nfse.IssueResult, error) {
	c, err := g.client(cert, env)
	if err != nil {
		return nfse.IssueResult{}, err
	}
	return c.Issue(ctx, signed)
}

// DPSKey consulta a DPS pelo Id.
func (g *Gateway) DPSKey(ctx context.Context, cert *certificate.Certificate, env int, id string) (string, bool, error) {
	c, err := g.client(cert, env)
	if err != nil {
		return "", false, err
	}
	return c.DPSKey(ctx, id)
}

// Get baixa a NFS-e.
func (g *Gateway) Get(ctx context.Context, cert *certificate.Certificate, env int, key string) ([]byte, bool, error) {
	c, err := g.client(cert, env)
	if err != nil {
		return nil, false, err
	}
	return c.Get(ctx, key)
}

// SendEvent envia o pedido de evento.
func (g *Gateway) SendEvent(ctx context.Context, cert *certificate.Certificate, env int, key string, signed []byte) (nfse.EventResult, error) {
	c, err := g.client(cert, env)
	if err != nil {
		return nfse.EventResult{}, err
	}
	return c.SendEvent(ctx, key, signed)
}

// EventRegistered consulta se a nota tem o evento (sequencial 1).
func (g *Gateway) EventRegistered(ctx context.Context, cert *certificate.Certificate, env int, key, kind string) (bool, error) {
	c, err := g.client(cert, env)
	if err != nil {
		return false, err
	}
	return c.EventRegistered(ctx, key, kind, 1)
}

// CancelState pergunta à Sefin pelos eventos de cancelamento da nota.
func (g *Gateway) CancelState(ctx context.Context, cert *certificate.Certificate, env int, key string) (string, error) {
	c, err := g.client(cert, env)
	if err != nil {
		return "", err
	}
	return c.CancelState(ctx, key)
}
