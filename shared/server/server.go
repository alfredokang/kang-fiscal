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
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice"
	invoiceout "github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/port/output"
	invoiceusecase "github.com/alfredokang/kang-fiscal/internal/modules/invoice/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule"
	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/helpers"
	"github.com/alfredokang/kang-fiscal/shared/logger"
	"github.com/alfredokang/kang-fiscal/shared/middleware"
	"github.com/alfredokang/kang-fiscal/shared/secretbox"
)

// Server é a API HTTP.
type Server struct {
	db     *sql.DB
	box    *secretbox.Box
	sefaz  invoiceout.SefazGateway
	router chi.Router
}

// Option ajusta o servidor (testes trocam a SEFAZ por uma falsa).
type Option func(*Server)

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

	r.Route("/v1", func(v1 chi.Router) {
		// Timeout só na API: a emissão vai esperar a SEFAZ, e um teto aqui impede que
		// uma autorizadora travada prenda conexões para sempre.
		v1.Use(chimw.Timeout(60 * time.Second))
		v1.Route("/admin", func(admin chi.Router) {
			admin.Use(middleware.AdminToken(config.Env.AdminToken))
			issuerModule.RegisterRoutes(admin)
			certificateModule.RegisterRoutes(admin)
			taxModule.RegisterRoutes(admin)
			numberingModule.RegisterRoutes(admin)
		})
		// A emissão fica fora de /admin porque é a rota dos sistemas integrados. Até as
		// credenciais por tenant existirem, ela usa o mesmo ADMIN_TOKEN.
		v1.Group(func(api chi.Router) {
			api.Use(middleware.AdminToken(config.Env.AdminToken))
			invoiceModule.RegisterRoutes(api)
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
