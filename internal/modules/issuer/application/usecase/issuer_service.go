// Package usecase orquestra os casos de uso do emitente.
package usecase

import (
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/port/output"
)

// IssuerService é a fachada do módulo: cada método vive no arquivo do seu caso de uso.
type IssuerService struct {
	repo output.IssuerRepository
	// now é injetável para os testes não dependerem do relógio.
	now func() time.Time
}

// NewIssuerService monta o serviço.
func NewIssuerService(repo output.IssuerRepository) *IssuerService {
	return &IssuerService{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}
