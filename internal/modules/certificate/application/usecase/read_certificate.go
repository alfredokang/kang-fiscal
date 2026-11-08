package usecase

import (
	"context"
	"encoding/base64"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/mapper"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/valueobject"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
)

// Get devolve os dados públicos do certificado ativo.
func (s *CertificateService) Get(ctx context.Context, q command.IssuerCertificate) (dto.CertificateDTO, error) {
	ref, err := vo.ParseIssuerRef(q.IssuerRef)
	if err != nil {
		return dto.CertificateDTO{}, err
	}
	cert, err := s.repo.FindActive(ctx, q.TenantID, ref)
	if err != nil {
		return dto.CertificateDTO{}, err
	}
	return mapper.ToDTO(cert, s.now()), nil
}

// Revoke desativa o certificado e apaga o segredo. A partir daqui o emitente não assina
// nada até receber outro.
func (s *CertificateService) Revoke(ctx context.Context, cmd command.IssuerCertificate) error {
	ref, err := vo.ParseIssuerRef(cmd.IssuerRef)
	if err != nil {
		return err
	}
	return s.repo.Revoke(ctx, cmd.TenantID, ref, s.now())
}

// Signer decifra o certificado ativo para assinar.
//
// O vencimento é conferido de novo aqui, e não só no upload: o certificado que entrou
// válido vence numa terça à noite, e a nota assinada com ele seria rejeitada.
func (s *CertificateService) Signer(ctx context.Context, q command.IssuerCertificate) (*certificate.Certificate, error) {
	ref, err := vo.ParseIssuerRef(q.IssuerRef)
	if err != nil {
		return nil, err
	}
	meta, sealed, err := s.repo.FindActiveSealed(ctx, q.TenantID, ref)
	if err != nil {
		return nil, err
	}
	if !meta.ValidAt(s.now()) {
		return nil, domain.ErrExpired
	}
	pfxB64, err := s.box.Open(sealed.PFX)
	if err != nil {
		return nil, err
	}
	password, err := s.box.Open(sealed.Password)
	if err != nil {
		return nil, err
	}
	pfx, err := base64.StdEncoding.DecodeString(pfxB64)
	if err != nil {
		return nil, err
	}
	return certificate.Load(pfx, password)
}
