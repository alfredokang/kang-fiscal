// Package mapper converte entre o agregado e a linha do banco.
package mapper

import (
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/entity"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/valueobject"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/infrastructure/persistence/postgres/model"
)

// ToModel converte o agregado.
func ToModel(c *entity.Certificate) model.CertificateModel {
	return model.CertificateModel{
		ID: string(c.ID), TenantID: c.TenantID, IssuerRef: string(c.IssuerRef),
		HolderName: c.HolderName, CNPJ: c.CNPJ, CAName: c.CAName, Serial: c.Serial,
		Fingerprint: c.Fingerprint, NotBefore: c.NotBefore, NotAfter: c.NotAfter, CreatedAt: c.CreatedAt,
	}
}

// ToEntity reconstrói o agregado.
func ToEntity(m model.CertificateModel) *entity.Certificate {
	return &entity.Certificate{
		ID: vo.CertificateID(m.ID), TenantID: m.TenantID, IssuerRef: vo.IssuerRef(m.IssuerRef),
		HolderName: m.HolderName, CNPJ: m.CNPJ, CAName: m.CAName, Serial: m.Serial,
		Fingerprint: m.Fingerprint, NotBefore: m.NotBefore, NotAfter: m.NotAfter, CreatedAt: m.CreatedAt,
	}
}
