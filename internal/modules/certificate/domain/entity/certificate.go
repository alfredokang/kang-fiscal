// Package entity contém o agregado do certificado do emitente.
package entity

import (
	"errors"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/valueobject"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
)

// ExpiryWarningDays é quando o aviso de vencimento começa. Trinta dias é o prazo realista
// para comprar e instalar um A1 novo: a AC pede validação presencial ou por vídeo, e a
// agenda costuma levar uma semana.
const ExpiryWarningDays = 30

// Certificate é o certificado A1 ativo de um emitente.
//
// O agregado guarda só o que é público do certificado. O .pfx e a senha nunca passam por
// aqui depois do upload: vão cifrados direto para o repositório (Constituição, Art. 9.1).
type Certificate struct {
	ID          vo.CertificateID
	TenantID    string
	IssuerRef   vo.IssuerRef
	HolderName  string
	CNPJ        string
	CAName      string
	Serial      string
	Fingerprint string
	NotBefore   time.Time
	NotAfter    time.Time
	CreatedAt   time.Time
}

// FromUpload abre o .pfx, confere tudo o que impede uma assinatura válida e devolve o
// agregado e o certificado aberto (para o chamador cifrar o arquivo).
//
// A conferência do CNPJ é pela raiz (8 primeiras posições): o A1 da matriz assina as notas
// das filiais, e exigir o CNPJ completo travaria toda empresa com mais de um endereço.
func FromUpload(tenantID string, issuer vo.IssuerRef, issuerCNPJ string, pfx []byte, password string, now time.Time) (*Certificate, error) {
	if len(pfx) == 0 {
		return nil, domain.ErrEmptyFile
	}
	if len(pfx) > certificate.MaxFileSize {
		return nil, domain.ErrFileTooLarge
	}
	cert, err := certificate.Load(pfx, password)
	switch {
	case errors.Is(err, certificate.ErrNotRSA):
		return nil, domain.ErrNotRSA
	case err != nil:
		return nil, domain.ErrWrongPassword
	}
	if err := cert.ValidAt(now); err != nil {
		return nil, domain.ErrExpired
	}
	cnpj := cert.CNPJ()
	if cnpj == "" {
		return nil, domain.ErrNoCNPJ
	}
	if len(issuerCNPJ) < 8 || cnpj[:8] != issuerCNPJ[:8] {
		return nil, domain.ErrCNPJMismatch
	}
	id, err := vo.NewCertificateID()
	if err != nil {
		return nil, err
	}
	return &Certificate{
		ID: id, TenantID: tenantID, IssuerRef: issuer,
		HolderName: cert.HolderName(), CNPJ: cnpj, CAName: cert.IssuerName(),
		Serial: cert.SerialNumber(), Fingerprint: cert.Fingerprint(),
		NotBefore: cert.NotBefore(), NotAfter: cert.NotAfter(), CreatedAt: now,
	}, nil
}

// DaysLeft são os dias até o vencimento, arredondados para baixo; negativo se venceu.
func (c *Certificate) DaysLeft(now time.Time) int {
	return int(c.NotAfter.Sub(now).Hours() / 24)
}

// ExpiringSoon diz se o aviso de vencimento deve aparecer.
func (c *Certificate) ExpiringSoon(now time.Time) bool {
	return c.DaysLeft(now) <= ExpiryWarningDays
}

// ValidAt diz se o certificado assina no instante dado.
func (c *Certificate) ValidAt(now time.Time) bool {
	return !now.Before(c.NotBefore) && !now.After(c.NotAfter)
}
