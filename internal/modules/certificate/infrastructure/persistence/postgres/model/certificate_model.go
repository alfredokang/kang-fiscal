// Package model é a linha de certificate_certificates, sem tags (Constituição, Art. 4.3).
package model

import "time"

// CertificateModel espelha as colunas públicas. Os segredos são lidos à parte, só por
// FindActiveSealed, para que nenhuma leitura comum os carregue sem querer.
type CertificateModel struct {
	ID          string    // id
	TenantID    string    // tenant_id
	IssuerRef   string    // issuer_ref
	HolderName  string    // holder_name
	CNPJ        string    // cnpj
	CAName      string    // ca_name
	Serial      string    // serial
	Fingerprint string    // fingerprint
	NotBefore   time.Time // not_before
	NotAfter    time.Time // not_after
	CreatedAt   time.Time // created_at
}
