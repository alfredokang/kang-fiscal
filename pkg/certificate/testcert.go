package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// TestPFX gera um PKCS#12 autoassinado no formato da ICP-Brasil (CN "NOME:CNPJ" e o CNPJ
// no otherName 2.16.76.1.3.3), para testes e para o ambiente de demonstração.
//
// Não serve para a SEFAZ — ela confere a cadeia ICP-Brasil e recusa o handshake. Serve
// para exercitar assinatura, cifra e cadastro sem ter um certificado real.
func TestPFX(holder, cnpj, password string, notAfter time.Time) ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	san, err := icpSAN(cnpj)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:    big.NewInt(time.Now().UnixNano()),
		Subject:         pkix.Name{CommonName: holder + ":" + cnpj, Organization: []string{"Kang Fiscal (teste)"}},
		NotBefore:       time.Now().Add(-time.Hour),
		NotAfter:        notAfter,
		KeyUsage:        x509.KeyUsageDigitalSignature,
		ExtKeyUsage:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		ExtraExtensions: []pkix.Extension{{Id: oidSubjectAltName, Value: san}},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return pkcs12.Modern.Encode(key, leaf, nil, password)
}

// icpSAN monta o subjectAltName com o otherName do CNPJ, como as ACs da ICP-Brasil fazem.
func icpSAN(cnpj string) ([]byte, error) {
	value, err := asn1.Marshal(cnpj) // PrintableString
	if err != nil {
		return nil, err
	}
	other, err := asn1.Marshal(struct {
		OID   asn1.ObjectIdentifier
		Value asn1.RawValue `asn1:"tag:0,explicit"`
	}{oidCNPJ, asn1.RawValue{FullBytes: value}})
	if err != nil {
		return nil, err
	}
	// O otherName é [0] IMPLICIT dentro do GeneralNames: troca a tag SEQUENCE por [0].
	var seq asn1.RawValue
	if _, err := asn1.Unmarshal(other, &seq); err != nil {
		return nil, err
	}
	name := asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: seq.Bytes}
	return asn1.Marshal([]asn1.RawValue{name})
}
