// Package mapper converte o agregado em DTO.
package mapper

import (
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/entity"
)

// ToDTO converte o agregado no instante dado (os dias que faltam dependem do relógio).
func ToDTO(c *entity.Certificate, now time.Time) dto.CertificateDTO {
	return dto.CertificateDTO{
		IssuerRef:    string(c.IssuerRef),
		HolderName:   c.HolderName,
		CNPJ:         c.CNPJ,
		CAName:       c.CAName,
		Serial:       c.Serial,
		Fingerprint:  c.Fingerprint,
		NotBefore:    c.NotBefore,
		NotAfter:     c.NotAfter,
		DaysLeft:     c.DaysLeft(now),
		ExpiringSoon: c.ExpiringSoon(now),
		CreatedAt:    c.CreatedAt,
	}
}
