// Package xmldsig assina e confere documentos fiscais no perfil de XML Signature da SEFAZ.
//
// O perfil é estreito e fixo, e é isso que o torna interoperável: assinatura enveloped,
// uma referência para o atributo Id do elemento assinado, Canonical XML 1.0 sem
// comentários, resumo SHA-1, RSA com PKCS#1 v1.5 e o certificado em X509Data. O Signature
// é sempre o último filho do elemento que contém o bloco assinado (na NF-e, o infNFe é
// assinado e o Signature entra no fim do NFe).
//
// O desenho segue github.com/mschunke/gonfe/xmldsig (MIT): a assinatura é inserida nos
// bytes originais, sem reserializar o documento, para o resumo calculado aqui ser o mesmo
// que a SEFAZ recalcula. A canonicalização é implementação própria.
package xmldsig

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // SHA-1 é o que o leiaute da NF-e exige; não é escolha.
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Algoritmos do perfil da SEFAZ.
const (
	AlgC14N      = "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"
	AlgEnveloped = "http://www.w3.org/2000/09/xmldsig#enveloped-signature"
	AlgDigest    = "http://www.w3.org/2000/09/xmldsig#sha1"
	AlgSignature = "http://www.w3.org/2000/09/xmldsig#rsa-sha1"

	// NamespaceDSig é o namespace do XML Signature.
	NamespaceDSig = "http://www.w3.org/2000/09/xmldsig#"
)

var (
	ErrElementNotFound = errors.New("xmldsig: elemento a assinar não encontrado")
	ErrNoID            = errors.New("xmldsig: o elemento a assinar não tem atributo Id")
	ErrAlreadySigned   = errors.New("xmldsig: o documento já está assinado")
	ErrNotSigned       = errors.New("xmldsig: o documento não está assinado")
	ErrInvalid         = errors.New("xmldsig: assinatura inválida")
	ErrAlgorithm       = errors.New("xmldsig: algoritmo fora do perfil da SEFAZ")
)

// Signer produz a assinatura. *certificate.Certificate implementa; um adaptador de A3 ou
// HSM também pode.
type Signer interface {
	Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error)
	DER() []byte
}

// Sign assina o primeiro elemento `tag` do documento e devolve o XML assinado.
func Sign(doc []byte, tag string, signer Signer) ([]byte, error) {
	root, err := parse(doc)
	if err != nil {
		return nil, err
	}
	target := root.find(tag, "")
	if target == nil {
		return nil, fmt.Errorf("%w: <%s>", ErrElementNotFound, tag)
	}
	id, ok := target.attr("Id")
	if !ok || id == "" {
		return nil, fmt.Errorf("%w: <%s>", ErrNoID, tag)
	}
	parent := target.parent
	if parent == root {
		return nil, fmt.Errorf("xmldsig: <%s> é a raiz; a assinatura precisa de um elemento que a contenha", tag)
	}
	if parent.endTagStart < 0 {
		return nil, fmt.Errorf("xmldsig: <%s> está na forma abreviada e não comporta a assinatura", parent.local)
	}
	for _, c := range parent.elements() {
		if c.local == "Signature" && c.namespace() == NamespaceDSig {
			return nil, ErrAlreadySigned
		}
	}

	sum := sha1.Sum(canonicalize(target, true)) //nolint:gosec
	digest := base64.StdEncoding.EncodeToString(sum[:])
	cert := signer.DER()
	if len(cert) == 0 {
		return nil, errors.New("xmldsig: o assinante não forneceu o certificado")
	}

	// Insere com o SignatureValue vazio, relê e canonicaliza o SignedInfo no lugar onde
	// ele vai ficar. Assim as declarações de namespace herdadas são as reais, em qualquer
	// documento — e não uma suposição sobre onde ele vai ser embutido.
	const placeholder = "\x00KANG-FISCAL-SIGNATURE\x00"
	block := `<Signature xmlns="` + NamespaceDSig + `">` + signedInfo(id, digest) +
		`<SignatureValue>` + placeholder + `</SignatureValue>` +
		`<KeyInfo><X509Data><X509Certificate>` + base64.StdEncoding.EncodeToString(cert) +
		`</X509Certificate></X509Data></KeyInfo></Signature>`
	pos := parent.endTagStart
	out := make([]byte, 0, len(doc)+len(block)+512)
	out = append(out, doc[:pos]...)
	out = append(out, block...)
	out = append(out, doc[pos:]...)

	// O placeholder tem \x00, que XML não aceita: troca por texto neutro só para reler.
	probe := []byte(strings.Replace(string(out), placeholder, "", 1))
	reparsed, err := parse(probe)
	if err != nil {
		return nil, err
	}
	si := reparsed.find("SignedInfo", NamespaceDSig)
	siSum := sha1.Sum(canonicalize(si, false)) //nolint:gosec
	raw, err := signer.Sign(rand.Reader, siSum[:], crypto.SHA1)
	if err != nil {
		return nil, fmt.Errorf("xmldsig: falha ao assinar: %w", err)
	}
	return []byte(strings.Replace(string(out), placeholder, base64.StdEncoding.EncodeToString(raw), 1)), nil
}

func signedInfo(id, digest string) string {
	return `<SignedInfo>` +
		`<CanonicalizationMethod Algorithm="` + AlgC14N + `"></CanonicalizationMethod>` +
		`<SignatureMethod Algorithm="` + AlgSignature + `"></SignatureMethod>` +
		`<Reference URI="#` + escapeAttr(id) + `">` +
		`<Transforms>` +
		`<Transform Algorithm="` + AlgEnveloped + `"></Transform>` +
		`<Transform Algorithm="` + AlgC14N + `"></Transform>` +
		`</Transforms>` +
		`<DigestMethod Algorithm="` + AlgDigest + `"></DigestMethod>` +
		`<DigestValue>` + digest + `</DigestValue>` +
		`</Reference>` +
		`</SignedInfo>`
}

// Verify confere todas as assinaturas do documento e devolve o certificado da primeira.
//
// A conferência é criptográfica e estrutural: resumo do elemento referenciado, assinatura
// do SignedInfo e algoritmos do perfil. Ela não estabelece confiança na cadeia ICP-Brasil
// nem consulta revogação — isso é do cliente da SEFAZ.
func Verify(doc []byte) (*x509.Certificate, error) {
	root, err := parse(doc)
	if err != nil {
		return nil, err
	}
	var sigs []*node
	root.findAll("Signature", NamespaceDSig, &sigs)
	if len(sigs) == 0 {
		return nil, ErrNotSigned
	}
	var first *x509.Certificate
	for _, sig := range sigs {
		cert, err := verifyOne(root, sig)
		if err != nil {
			return nil, err
		}
		if first == nil {
			first = cert
		}
	}
	return first, nil
}

func verifyOne(root, sig *node) (*x509.Certificate, error) {
	si := sig.child("SignedInfo")
	if si == nil {
		return nil, fmt.Errorf("%w: sem SignedInfo", ErrInvalid)
	}
	ref := si.child("Reference")
	if ref == nil {
		return nil, fmt.Errorf("%w: sem Reference", ErrInvalid)
	}
	if err := checkAlgorithms(si, ref); err != nil {
		return nil, err
	}
	uri, _ := ref.attr("URI")
	id := strings.TrimPrefix(uri, "#")
	target := root.byID(id)
	if id == "" || target == nil {
		return nil, fmt.Errorf("%w: nenhum elemento com Id=%q", ErrInvalid, id)
	}
	dv := ref.child("DigestValue")
	if dv == nil {
		return nil, fmt.Errorf("%w: sem DigestValue", ErrInvalid)
	}
	sum := sha1.Sum(canonicalize(target, true)) //nolint:gosec
	if base64.StdEncoding.EncodeToString(sum[:]) != strings.TrimSpace(dv.textContent()) {
		return nil, fmt.Errorf("%w: o resumo de <%s> não confere", ErrInvalid, target.local)
	}

	certNode := sig.find("X509Certificate", NamespaceDSig)
	if certNode == nil {
		return nil, fmt.Errorf("%w: sem X509Certificate", ErrInvalid)
	}
	der, err := base64.StdEncoding.DecodeString(stripSpace(certNode.textContent()))
	if err != nil {
		return nil, fmt.Errorf("%w: X509Certificate ilegível", ErrInvalid)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("%w: X509Certificate ilegível", ErrInvalid)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: chave pública não é RSA", ErrAlgorithm)
	}
	sv := sig.child("SignatureValue")
	if sv == nil {
		return nil, fmt.Errorf("%w: sem SignatureValue", ErrInvalid)
	}
	value, err := base64.StdEncoding.DecodeString(stripSpace(sv.textContent()))
	if err != nil {
		return nil, fmt.Errorf("%w: SignatureValue ilegível", ErrInvalid)
	}
	siSum := sha1.Sum(canonicalize(si, false)) //nolint:gosec
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA1, siSum[:], value); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return cert, nil
}

func checkAlgorithms(si, ref *node) error {
	expect := func(n *node, want, what string) error {
		if n == nil {
			return fmt.Errorf("%w: sem %s", ErrInvalid, what)
		}
		if got, _ := n.attr("Algorithm"); got != want {
			return fmt.Errorf("%w: %s usa %q", ErrAlgorithm, what, got)
		}
		return nil
	}
	if err := expect(si.child("CanonicalizationMethod"), AlgC14N, "CanonicalizationMethod"); err != nil {
		return err
	}
	if err := expect(si.child("SignatureMethod"), AlgSignature, "SignatureMethod"); err != nil {
		return err
	}
	return expect(ref.child("DigestMethod"), AlgDigest, "DigestMethod")
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		}
		return r
	}, s)
}
