// Package request são os corpos HTTP aceitos pelo módulo.
package request

// UploadCertificateRequest é o PUT /v1/admin/issuers/{id}/certificate.
//
// O .pfx vai em base64 dentro do JSON, e não em multipart: é o formato que todo cliente
// HTTP de toda linguagem monta sem biblioteca extra. String e GoString omitem o conteúdo.
type UploadCertificateRequest struct {
	PFXBase64 string `json:"pfx_base64"`
	Password  string `json:"password"`
}

func (UploadCertificateRequest) String() string   { return "[certificado omitido]" }
func (UploadCertificateRequest) GoString() string { return "[certificado omitido]" }
