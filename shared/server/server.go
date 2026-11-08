// Package server é o composition root: o único lugar que enxerga todos os módulos ao mesmo
// tempo (Constituição, Art. 1.2).
package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate"
	"github.com/alfredokang/kang-fiscal/internal/modules/credential"
	"github.com/alfredokang/kang-fiscal/internal/modules/dfe"
	dfeout "github.com/alfredokang/kang-fiscal/internal/modules/dfe/application/port/output"
	dfeusecase "github.com/alfredokang/kang-fiscal/internal/modules/dfe/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice"
	invoiceout "github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/port/output"
	invoiceusecase "github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer"
	"github.com/alfredokang/kang-fiscal/internal/modules/nfse"
	nfseout "github.com/alfredokang/kang-fiscal/internal/modules/nfse/application/port/output"
	nfseusecase "github.com/alfredokang/kang-fiscal/internal/modules/nfse/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/logger"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
	"github.com/alfredokang/kang-fiscal/shared/openapi"
	"github.com/alfredokang/kang-fiscal/shared/secretbox"
)

// Server é a API HTTP.
type Server struct {
	invoices    *invoiceusecase.InvoiceService
	dfe         *dfeusecase.DFeService
	dfeGateway  dfeout.Gateway
	nfseGateway nfseout.Gateway
	nfse        *nfseusecase.Service
	db          *sql.DB
	box         *secretbox.Box
	sefaz       invoiceout.SefazGateway
	router      chi.Router
}

// Option ajusta o servidor (testes trocam a SEFAZ por uma falsa).
type Option func(*Server)

// WithDFeGateway troca o gateway do Ambiente Nacional (testes).
func WithDFeGateway(g dfeout.Gateway) Option { return func(s *Server) { s.dfeGateway = g } }

// WithNFSeGateway troca o gateway da Sefin Nacional (testes).
func WithNFSeGateway(g nfseout.Gateway) Option { return func(s *Server) { s.nfseGateway = g } }

// NFSe devolve o serviço da NFS-e (o worker resolve as incertas).
func (s *Server) NFSe() *nfseusecase.Service { return s.nfse }

// DFe devolve o serviço de recebimento (o worker sincroniza).
func (s *Server) DFe() *dfeusecase.DFeService { return s.dfe }

// WithSefaz troca o gateway da SEFAZ.
func WithSefaz(g invoiceout.SefazGateway) Option { return func(s *Server) { s.sefaz = g } }

// NewServer monta os módulos e as rotas. Chave de cifra inválida derruba o boot: subir
// com a chave errada faria todo certificado já gravado parecer corrompido.
func NewServer(db *sql.DB, opts ...Option) (*Server, error) {
	box, err := secretbox.New(config.Env.SecretsEncryptionKey)
	if err != nil {
		return nil, err
	}
	s := &Server{db: db, box: box}
	for _, o := range opts {
		o(s)
	}
	s.router = s.routes()
	return s, nil
}

// Invoices devolve o serviço de emissão (o worker usa a mesma montagem da API).
func (s *Server) Invoices() *invoiceusecase.InvoiceService { return s.invoices }

// Handler devolve o roteador (testes de ponta a ponta).
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	if logger.Logger != nil {
		r.Use(logger.ZapLoggerMiddleware)
	}

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		helpers.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/ready", s.ready)
	r.Handle("/metrics", promhttp.Handler())
	openapi.Register(r)

	credentialModule := credential.NewModule(s.db, config.Env.CredentialPepper)
	issuerModule := issuer.NewModule(s.db)
	certificateModule := certificate.NewModule(s.db, s.box,
		certificateIssuerAdapter{issuers: issuerModule.IssuerService})
	taxModule := taxrule.NewModule(s.db)
	numberingModule := numbering.NewModule(s.db, numberingIssuerAdapter{issuers: issuerModule.IssuerService})
	invoiceModule := invoice.NewModule(s.db, invoiceusecase.Deps{
		Issuers:  invoiceIssuerAdapter{issuers: issuerModule.IssuerService},
		Signers:  invoiceSignerAdapter{certs: certificateModule.CertificateService},
		Profiles: invoiceTaxAdapter{taxes: taxModule.Service},
		Rates:    invoiceTaxAdapter{taxes: taxModule.Service},
		Numbers:  invoiceNumberAdapter{numbers: numberingModule.Service},
		Sefaz:    s.sefaz,
		RespTec:  respTec(),
	})
	s.invoices = invoiceModule.Service
	dfeModule := dfe.NewModule(s.db, dfeAdapter{issuers: issuerModule.IssuerService, certs: certificateModule.CertificateService},
		dfeAdapter{issuers: issuerModule.IssuerService, certs: certificateModule.CertificateService}, s.dfeGateway)
	s.dfe = dfeModule.Service
	nfseDeps := nfseAdapter{issuers: issuerModule.IssuerService, certs: certificateModule.CertificateService, numbers: numberingModule.Service}
	nfseModule := nfse.NewModule(s.db, nfseDeps, nfseDeps, nfseDeps, s.nfseGateway)
	s.nfse = nfseModule.Service

	r.Route("/v1", func(v1 chi.Router) {
		// Timeout só na API: a emissão vai esperar a SEFAZ, e um teto aqui impede que
		// uma autorizadora travada prenda conexões para sempre.
		v1.Use(chimw.Timeout(60 * time.Second))
		// /v1/admin: só o ADMIN_TOKEN. Administra a instalação (credenciais, alíquotas da
		// reforma, varredura) e mantém o acesso completo ao tenant default — quem roda o
		// Kang Fiscal para a própria empresa não precisa de credencial nenhuma.
		v1.Route("/admin", func(admin chi.Router) {
			admin.Use(middleware.AdminToken(config.Env.AdminToken))
			credentialModule.RegisterAdminRoutes(admin)
			issuerModule.RegisterRoutes(admin)
			certificateModule.RegisterRoutes(admin)
			taxModule.RegisterRoutes(admin)
			numberingModule.RegisterRoutes(admin)
			invoiceModule.RegisterAdminRoutes(admin)
		})
		// /v1: as rotas dos sistemas integrados. Credencial do tenant (ou o ADMIN_TOKEN, no
		// tenant default), escopo pela tabela scopeFor e limite por credencial.
		v1.Group(func(api chi.Router) {
			api.Use(middleware.Authenticate(config.Env.AdminToken, credentialResolver(credentialModule.Service)))
			api.Use(middleware.RateLimit(config.Env.RateLimitPerMinute))
			api.Use(middleware.RequireScope(scopeFor))
			issuerModule.RegisterRoutes(api)
			certificateModule.RegisterRoutes(api)
			taxModule.RegisterTenantRoutes(api)
			numberingModule.RegisterRoutes(api)
			invoiceModule.RegisterRoutes(api)
			dfeModule.RegisterRoutes(api)
			nfseModule.RegisterRoutes(api)
		})
	})
	return r
}

// respTec é o responsável técnico pelo sistema emissor (infRespTec), exigido em várias UFs.
// Quem instala o Kang Fiscal para clientes informa o próprio CNPJ; vazio, o grupo não vai.
func respTec() *nfe.InfRespTec {
	e := config.Env
	if e.RespTecCNPJ == "" {
		return nil
	}
	return &nfe.InfRespTec{CNPJ: e.RespTecCNPJ, XContato: e.RespTecContact, Email: e.RespTecEmail, Fone: e.RespTecPhone}
}

// ready responde se o serviço pode receber tráfego: o Postgres precisa responder.
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		helpers.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "postgres indisponível"})
		return
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// Start sobe o HTTP e espera SIGINT/SIGTERM para desligar sem cortar requisição no meio.
//
// Desligar no meio de uma emissão é exatamente o cenário do Art. 6.1: a nota pode ter
// chegado à SEFAZ sem a resposta chegar aqui. O desligamento espera as requisições em
// andamento terminarem antes de fechar.
func (s *Server) Start() error {
	srv := &http.Server{
		Addr:              ":" + config.Env.ApiPort,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.SugaredLogger.Infof("Kang Fiscal ouvindo em :%s", config.Env.ApiPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-stop:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	logger.SugaredLogger.Info("desligando: esperando as requisições em andamento")
	return srv.Shutdown(ctx)
}
