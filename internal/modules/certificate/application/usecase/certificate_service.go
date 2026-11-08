// Package usecase orquestra os casos de uso do certificado.
package usecase

import (
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/port/output"
)

// CertificateService é a fachada do módulo: cada método vive no arquivo do seu caso de uso.
type CertificateService struct {
	repo    output.CertificateRepository
	box     output.SecretBox
	issuers output.IssuerReader
	now     func() time.Time
}

// NewCertificateService monta o serviço.
func NewCertificateService(repo output.CertificateRepository, box output.SecretBox, issuers output.IssuerReader) *CertificateService {
	return &CertificateService{repo: repo, box: box, issuers: issuers, now: func() time.Time { return time.Now().UTC() }}
}
