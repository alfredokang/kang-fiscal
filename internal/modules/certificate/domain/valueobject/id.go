// Package valueobject contém os valores do módulo de certificado.
package valueobject

import (
	"strings"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
)

// CertificateID é o UUID v7 do registro do certificado.
type CertificateID string

// NewCertificateID gera um id novo.
func NewCertificateID() (CertificateID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return CertificateID(id.String()), nil
}

// IssuerRef é o id do emitente como texto opaco: o emitente é de outro módulo (Art. 2).
type IssuerRef string

// ParseIssuerRef confere o formato do id recebido na URL.
func ParseIssuerRef(raw string) (IssuerRef, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", domain.ErrInvalidID
	}
	return IssuerRef(id.String()), nil
}
