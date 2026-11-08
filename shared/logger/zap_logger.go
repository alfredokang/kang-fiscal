// Package logger é o log estruturado do serviço.
package logger

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/alfredokang/kang-fiscal/shared/config"
)

var (
	Logger        *zap.Logger
	SugaredLogger *zap.SugaredLogger
)

// InitLogger inicializa o logger estruturado.
//
// Produção sai em JSON (para ingestão em Loki/CloudWatch); desenvolvimento sai legível.
// Nunca se loga PII, certificado nem segredo — Constituição, Art. 10.
func InitLogger() error {
	var err error
	if config.IsProduction() {
		Logger, err = zap.NewProduction()
	} else {
		Logger, err = zap.NewDevelopment()
	}
	if err != nil {
		return err
	}
	SugaredLogger = Logger.Sugar()
	return nil
}

// ZapLoggerMiddleware registra cada requisição com o req_id do chi.
//
// Não registra corpo nem query string: o corpo de uma emissão traz CPF e endereço do
// destinatário, e a query de uma consulta pode trazer o documento.
func ZapLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Logger.Info("http_request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("req_id", middleware.GetReqID(r.Context())),
			zap.String("remote_addr", r.RemoteAddr),
		)
		next.ServeHTTP(w, r)
	})
}
