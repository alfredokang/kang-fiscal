package usecase

import (
	"context"
	"encoding/base64"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/mapper"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/entity"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/valueobject"
)

// Upload confere o .pfx contra o emitente, cifra o arquivo e a senha e troca o ativo.
func (s *CertificateService) Upload(ctx context.Context, cmd command.UploadCertificate) (dto.CertificateDTO, error) {
	// Sem chave, recusa antes de abrir o arquivo: guardar em claro não é alternativa.
	if !s.box.Ready() {
		return dto.CertificateDTO{}, domain.ErrEncryptionDisabled
	}
	ref, err := vo.ParseIssuerRef(cmd.IssuerRef)
	if err != nil {
		return dto.CertificateDTO{}, err
	}
	issuer, err := s.issuers.IssuerForCertificate(ctx, cmd.TenantID, string(ref))
	if err != nil {
		return dto.CertificateDTO{}, err
	}
	now := s.now()
	cert, err := entity.FromUpload(cmd.TenantID, ref, issuer.CNPJ, cmd.PFX, cmd.Password, now)
	if err != nil {
		return dto.CertificateDTO{}, err
	}

	sealedPFX, err := s.box.Seal(base64.StdEncoding.EncodeToString(cmd.PFX))
	if err != nil {
		return dto.CertificateDTO{}, err
	}
	sealedPassword, err := s.box.Seal(cmd.Password)
	if err != nil {
		return dto.CertificateDTO{}, err
	}
	if err := s.repo.Replace(ctx, cert, output.Sealed{PFX: sealedPFX, Password: sealedPassword}); err != nil {
		return dto.CertificateDTO{}, err
	}
	return mapper.ToDTO(cert, now), nil
}
