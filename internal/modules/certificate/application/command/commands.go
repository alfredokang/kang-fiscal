// Package command são as intenções de escrita do módulo.
package command

// UploadCertificate troca o certificado ativo do emitente.
//
// String e GoString omitem o arquivo e a senha: o command atravessa camadas, e um `%v`
// num log de erro não pode levar o certificado junto (Constituição, Art. 9.1).
type UploadCertificate struct {
	TenantID  string
	IssuerRef string
	PFX       []byte
	Password  string
}

func (UploadCertificate) String() string   { return "UploadCertificate{[certificado omitido]}" }
func (UploadCertificate) GoString() string { return "UploadCertificate{[certificado omitido]}" }

// IssuerCertificate identifica o certificado de um emitente.
type IssuerCertificate struct {
	TenantID  string
	IssuerRef string
}
