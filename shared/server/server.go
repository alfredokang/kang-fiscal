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
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer"
	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule"
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
	router chi.Router
}

// NewServer monta os módulos e as rotas. Chave de cifra inválida derruba o boot: subir
// com a chave errada faria todo certificado já gravado parecer corrompido.
func NewServer(db *sql.DB) (*Server, error) {
	box, err := secretbox.New(config.Env.SecretsEncryptionKey)
	if err != nil {
		return nil, err
	}
	s := &Server{db: db, box: box}
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

	r.Route("/v1", func(v1 chi.Router) {
		// Timeout só na API: a emissão vai esperar a SEFAZ, e um teto aqui impede que
		// uma autorizadora travada prenda conexões para sempre.
		v1.Use(chimw.Timeout(60 * time.Second))
		v1.Route("/admin", func(admin chi.Router) {
			admin.Use(middleware.AdminToken(config.Env.AdminToken))
			issuerModule.RegisterRoutes(admin)
			certificateModule.RegisterRoutes(admin)
			taxModule.RegisterRoutes(admin)
		})
	})
	return r
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
