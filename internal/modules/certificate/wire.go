// Package certificate monta o módulo do certificado A1.
package certificate

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/alfredokang/kang-fiscal/internal/modules/certificate/adapter/http"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/adapter/http/handler"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/usecase"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/infrastructure/persistence/postgres"
)

// Module expõe o que o composition root precisa. CertificateService é público porque a
// emissão vai pedir o assinante por porta (input.SignerProvider).
type Module struct {
	CertificateService *usecase.CertificateService
	handler            *handler.CertificateHandler
}

// NewModule monta a árvore de dependências. issuers lê o emitente, que é de outro módulo
// (Art. 1.2); box é a cifra com a chave do ambiente.
func NewModule(db *sql.DB, box output.SecretBox, issuers output.IssuerReader) *Module {
	service := usecase.NewCertificateService(postgres.NewCertificateRepository(db), box, issuers)
	return &Module{CertificateService: service, handler: handler.NewCertificateHandler(service)}
}

// RegisterRoutes pendura as rotas do módulo.
func (m *Module) RegisterRoutes(admin chi.Router) {
	adapterhttp.RegisterAdminRoutes(admin, m.handler)
}
