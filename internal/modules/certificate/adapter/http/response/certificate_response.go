// Package response são os corpos HTTP devolvidos pelo módulo. Nunca o arquivo nem a senha.
package response

import "time"

// CertificateResponse são os dados públicos do certificado ativo.
type CertificateResponse struct {
	IssuerID     string    `json:"issuer_id"`
	HolderName   string    `json:"holder_name"`
	CNPJ         string    `json:"cnpj"`
	CAName       string    `json:"ca_name"`
	Serial       string    `json:"serial"`
	Fingerprint  string    `json:"fingerprint_sha256"`
	NotBefore    time.Time `json:"not_before"`
	NotAfter     time.Time `json:"not_after"`
	DaysLeft     int       `json:"days_left"`
	ExpiringSoon bool      `json:"expiring_soon"`
	CreatedAt    time.Time `json:"created_at"`
}
