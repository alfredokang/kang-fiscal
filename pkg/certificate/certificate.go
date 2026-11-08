// Package certificate carrega e inspeciona o certificado digital ICP-Brasil A1 (.pfx/.p12).
//
// Adaptado de github.com/mschunke/gonfe/certificado — Copyright (c) 2026 mschunke, licença
// MIT. Mudanças: identificadores em inglês, CNPJ alfanumérico preservado (o original
// mantinha só dígitos, o que corta as letras do CNPJ de julho de 2026), e o tipo nunca se
// imprime em log (Constituição, Art. 9.1).
package certificate

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

var (
	// ErrWrongPassword: o PKCS#12 não abre — senha errada ou arquivo corrompido. Os dois
	// casos são o mesmo erro de propósito: distinguir ajudaria quem testa senhas.
	ErrWrongPassword = errors.New("certificado: senha incorreta ou arquivo corrompido")
	// ErrNotRSA: a assinatura da NF-e exige RSA.
	ErrNotRSA = errors.New("certificado: a chave privada não é RSA")
	// ErrExpired: fora do período de validade.
	ErrExpired = errors.New("certificado: fora do período de validade")
	// ErrNoLeaf: o arquivo não traz o certificado do titular.
	ErrNoLeaf = errors.New("certificado: o arquivo não traz o certificado do titular")
)

// OIDs da ICP-Brasil no subjectAltName (otherName).
var (
	oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
	// Pessoa física: data de nascimento (8) + CPF (11) + NIS + RG.
	oidNaturalPerson = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 1}
	// CNPJ da pessoa jurídica titular.
	oidCNPJ = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 3}
	// Responsável pela pessoa jurídica: data de nascimento (8) + CPF (11) + ...
	oidResponsible = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 4}
)

// Certificate é o certificado do titular com a chave privada.
//
// A chave fica em campo privado e o tipo implementa String/GoString: um `%v` acidental
// num log imprime "[certificado omitido]", nunca a chave.
type Certificate struct {
	leaf  *x509.Certificate
	chain []*x509.Certificate
	key   *rsa.PrivateKey
}

// Load decifra um PKCS#12 em memória. Aceita os algoritmos antigos (3DES/SHA-1) e os
// modernos (AES-256 com PBES2) que as autoridades certificadoras emitem hoje.
func Load(data []byte, password string) (*Certificate, error) {
	key, leaf, chain, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		return nil, ErrWrongPassword
	}
	return New(key, leaf, chain...)
}

// LoadFrom lê o PKCS#12 de um io.Reader.
func LoadFrom(r io.Reader, password string) (*Certificate, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileSize {
		return nil, ErrWrongPassword
	}
	return Load(data, password)
}

// MaxFileSize é o teto de um .pfx. Um A1 real tem poucos KB; o teto barra o arquivo que
// só existe para consumir memória do servidor.
const MaxFileSize = 64 << 10

// New monta o certificado a partir de chave e certificado já carregados (útil para chave
// guardada em outro lugar).
func New(key crypto.PrivateKey, leaf *x509.Certificate, chain ...*x509.Certificate) (*Certificate, error) {
	if leaf == nil {
		return nil, ErrNoLeaf
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrNotRSA
	}
	return &Certificate{leaf: leaf, chain: chain, key: rsaKey}, nil
}

// String nunca imprime a chave.
func (c *Certificate) String() string { return "[certificado omitido]" }

// GoString nunca imprime a chave, nem com %#v.
func (c *Certificate) GoString() string { return "[certificado omitido]" }

// Leaf devolve o certificado do titular.
func (c *Certificate) Leaf() *x509.Certificate { return c.leaf }

// Chain devolve os intermediários presentes no arquivo.
func (c *Certificate) Chain() []*x509.Certificate { return c.chain }

// CommonName é o CN, que na ICP-Brasil traz "RAZÃO SOCIAL:CNPJ".
func (c *Certificate) CommonName() string { return c.leaf.Subject.CommonName }

// HolderName é o CN sem o sufixo ":documento".
func (c *Certificate) HolderName() string {
	name := c.CommonName()
	if i := strings.LastIndex(name, ":"); i >= 0 && isDocument(strings.TrimSpace(name[i+1:])) {
		return strings.TrimSpace(name[:i])
	}
	return name
}

// CNPJ devolve o CNPJ do titular (14 posições, letras em maiúsculas), ou "" se for de
// pessoa física. Procura no otherName da ICP-Brasil e, na falta, no sufixo do CN.
func (c *Certificate) CNPJ() string {
	if v := c.otherName(oidCNPJ); len(v) >= 14 {
		return v[:14]
	}
	if s := c.cnSuffix(); len(s) == 14 {
		return s
	}
	return ""
}

// CPF devolve o CPF do titular pessoa física, ou "".
func (c *Certificate) CPF() string {
	if v := c.otherName(oidNaturalPerson); len(v) >= 19 {
		return v[8:19]
	}
	if s := c.cnSuffix(); len(s) == 11 {
		return s
	}
	return ""
}

// ResponsibleCPF é o CPF do responsável pela pessoa jurídica titular.
func (c *Certificate) ResponsibleCPF() string {
	if v := c.otherName(oidResponsible); len(v) >= 19 {
		return v[8:19]
	}
	return ""
}

// IssuerName é o CN da autoridade certificadora.
func (c *Certificate) IssuerName() string { return c.leaf.Issuer.CommonName }

// SerialNumber em hexadecimal.
func (c *Certificate) SerialNumber() string { return fmt.Sprintf("%X", c.leaf.SerialNumber) }

// Fingerprint é o SHA-256 do certificado em DER: identifica o certificado sem expor nada.
func (c *Certificate) Fingerprint() string {
	sum := sha256.Sum256(c.leaf.Raw)
	return hex.EncodeToString(sum[:])
}

// NotBefore é o início da validade.
func (c *Certificate) NotBefore() time.Time { return c.leaf.NotBefore }

// NotAfter é o fim da validade.
func (c *Certificate) NotAfter() time.Time { return c.leaf.NotAfter }

// ValidAt confere a validade no instante dado.
func (c *Certificate) ValidAt(t time.Time) error {
	if t.Before(c.leaf.NotBefore) {
		return fmt.Errorf("%w: só vale a partir de %s", ErrExpired, c.leaf.NotBefore.Format("02/01/2006"))
	}
	if t.After(c.leaf.NotAfter) {
		return fmt.Errorf("%w: venceu em %s", ErrExpired, c.leaf.NotAfter.Format("02/01/2006"))
	}
	return nil
}

// DER é o certificado do titular em DER — o conteúdo do X509Certificate da assinatura.
func (c *Certificate) DER() []byte { return c.leaf.Raw }

// TLS monta o par para a autenticação mútua com a SEFAZ, com a cadeia: algumas
// autorizadoras recusam o handshake sem os intermediários.
func (c *Certificate) TLS() tls.Certificate {
	chain := make([][]byte, 0, 1+len(c.chain))
	chain = append(chain, c.leaf.Raw)
	for _, ca := range c.chain {
		chain = append(chain, ca.Raw)
	}
	return tls.Certificate{Certificate: chain, PrivateKey: c.key, Leaf: c.leaf}
}

// Public implementa crypto.Signer.
func (c *Certificate) Public() crypto.PublicKey { return c.leaf.PublicKey }

// Sign implementa crypto.Signer com PKCS#1 v1.5, o esquema da assinatura da NF-e.
func (c *Certificate) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return rsa.SignPKCS1v15(rand, c.key, opts.HashFunc(), digest)
}

func (c *Certificate) cnSuffix() string {
	name := c.CommonName()
	i := strings.LastIndex(name, ":")
	if i < 0 {
		return ""
	}
	s := strings.ToUpper(strings.TrimSpace(name[i+1:]))
	if !isDocument(s) {
		return ""
	}
	return s
}

// otherName devolve o valor registrado sob o OID, só com dígitos e letras maiúsculas.
func (c *Certificate) otherName(oid asn1.ObjectIdentifier) string {
	for _, ext := range c.leaf.Extensions {
		if !ext.Id.Equal(oidSubjectAltName) {
			continue
		}
		var names []asn1.RawValue
		if _, err := asn1.Unmarshal(ext.Value, &names); err != nil {
			return ""
		}
		for _, rv := range names {
			if rv.Class != asn1.ClassContextSpecific || rv.Tag != 0 {
				continue
			}
			var other struct {
				OID   asn1.ObjectIdentifier
				Value asn1.RawValue `asn1:"tag:0,explicit"`
			}
			if _, err := asn1.UnmarshalWithParams(rv.FullBytes, &other, "tag:0"); err != nil {
				continue
			}
			if other.OID.Equal(oid) {
				return alnum(string(primitive(other.Value.Bytes)))
			}
		}
	}
	return ""
}

// primitive descasca camadas DER até o conteúdo: cada AC embrulha o valor de um jeito
// (PrintableString, UTF8String, OCTET STRING), e sem descascar os bytes de tag e tamanho
// se confundem com o documento.
func primitive(data []byte) []byte {
	for range 4 {
		var rv asn1.RawValue
		rest, err := asn1.Unmarshal(data, &rv)
		if err != nil || len(rest) != 0 || len(rv.Bytes) == 0 {
			return data
		}
		data = rv.Bytes
		if !rv.IsCompound {
			return data
		}
	}
	return data
}

// alnum mantém dígitos e letras. Letra é dado no CNPJ alfanumérico.
func alnum(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isDocument aceita 11 dígitos (CPF) ou 14 posições alfanuméricas (CNPJ).
func isDocument(s string) bool {
	if len(s) != 11 && len(s) != 14 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || len(s) == 14 && c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}
