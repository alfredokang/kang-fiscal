// Package dto é como o módulo devolve o certificado. Nunca carrega o arquivo nem a senha.
package dto

import "time"

// CertificateDTO são os dados públicos do certificado.
type CertificateDTO struct {
	IssuerRef    string
	HolderName   string
	CNPJ         string
	CAName       string
	Serial       string
	Fingerprint  string
	NotBefore    time.Time
	NotAfter     time.Time
	DaysLeft     int
	ExpiringSoon bool
	CreatedAt    time.Time
}
