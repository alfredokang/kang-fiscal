// Package nfce monta o que a NFC-e (modelo 65) tem além da NF-e: o QR Code e a URL de
// consulta por chave (grupo infNFeSupl).
//
// O QR Code é o da versão 3 (NT 2025.001). Na emissão online ele é só
// URL?p=chave|3|tpAmb — sem o hash do CSC que a versão 2 exigia. A v3 é obrigatória para
// consumidor pessoa física e para contingência, e aceita para pessoa jurídica; a v2 fica de
// fora de propósito: ela exige guardar o CSC, e um segredo a menos é um vazamento a menos.
package nfce

import (
	"crypto"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/alfredokang/kang-fiscal/pkg/nfe"
)

//go:embed urls.json
var urlsJSON []byte

// ErrNoURL: a UF não tem URL de QR Code no ambiente.
var ErrNoURL = errors.New("nfce: sem URL de QR Code para esta UF e ambiente")

var (
	once sync.Once
	urls struct {
		QRCode   map[string]map[string]string `json:"qr_code"`
		URLChave map[string]map[string]string `json:"url_chave"`
	}
)

func env(tpAmb int) string {
	if tpAmb == 1 {
		return "production"
	}
	return "homologation"
}

// URLs devolve a URL base do QR Code e a URL de consulta por chave da UF.
func URLs(uf string, tpAmb int) (qr, chave string, err error) {
	once.Do(func() { _ = json.Unmarshal(urlsJSON, &urls) })
	uf = strings.ToUpper(uf)
	qr = urls.QRCode[env(tpAmb)][uf]
	chave = urls.URLChave[env(tpAmb)][uf]
	if qr == "" || chave == "" {
		return "", "", ErrNoURL
	}
	return withScheme(qr), withScheme(chave), nil
}

// withScheme põe https:// quando a tabela traz só o host: o XSD exige o esquema na URL do
// QR Code.
func withScheme(u string) string {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	return "https://" + u
}

// QRCodeV3 é o conteúdo do QR Code da emissão online: URL?p=chave|3|tpAmb.
func QRCodeV3(baseURL, accessKey string, tpAmb int) string {
	sep := "?"
	if strings.Contains(baseURL, "?") {
		sep = "&"
	}
	return baseURL + sep + "p=" + accessKey + "|3|" + string(rune('0'+tpAmb))
}

// Supplement monta o infNFeSupl de uma NFC-e emitida online.
func Supplement(uf, accessKey string, tpAmb int) (*nfe.InfNFeSupl, error) {
	qr, chave, err := URLs(uf, tpAmb)
	if err != nil {
		return nil, err
	}
	return &nfe.InfNFeSupl{QRCode: QRCodeV3(qr, accessKey, tpAmb), URLChave: chave}, nil
}

// Signer assina com o certificado do emitente (RSA PKCS#1 v1.5). *certificate.Certificate
// implementa.
type Signer interface {
	Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error)
}

// Offline é o que entra no QR Code da NFC-e emitida em contingência offline (tpEmis 9).
type Offline struct {
	AccessKey string
	TpAmb     int
	Day       int    // dia da emissão (dhEmi), de 1 a 31
	VNF       string // valor total com 2 casas, como no XML ("19.50")
	// Destinatário: CNPJ (1), CPF (2) ou nenhum. Estrangeiro e não identificado vão vazios.
	CNPJ, CPF string
}

// QRCodeV3Offline monta URL?p=chave|3|tpAmb|dia|vNF|tp_idDest|idDest|assinatura.
//
// A assinatura é RSA com SHA-1 (PKCS#1 v1.5), em Base64, sobre os sete primeiros
// parâmetros com os separadores — com o mesmo certificado que assina a NFC-e (NT 2025.001
// e Manual de Padrões Técnicos do DANFE NFC-e 6.00; conferido contra a implementação do
// sped-nfe, que usa openssl_sign com SHA-1). Sem o CSC: é a assinatura que prova que o QR
// Code impresso na contingência é do emitente.
func QRCodeV3Offline(baseURL string, o Offline, signer Signer) (string, error) {
	tpID, id := "", ""
	switch {
	case o.CNPJ != "":
		tpID, id = "1", o.CNPJ
	case o.CPF != "":
		tpID, id = "2", o.CPF
	}
	params := fmt.Sprintf("%s|3|%d|%02d|%s|%s|%s", o.AccessKey, o.TpAmb, o.Day, o.VNF, tpID, id)
	sum := sha1.Sum([]byte(params)) //nolint:gosec // SHA-1 é o que a NT exige
	sig, err := signer.Sign(rand.Reader, sum[:], crypto.SHA1)
	if err != nil {
		return "", fmt.Errorf("nfce: assinatura do QR Code: %w", err)
	}
	sep := "?"
	if strings.Contains(baseURL, "?") {
		sep = "&"
	}
	return baseURL + sep + "p=" + params + "|" + base64.StdEncoding.EncodeToString(sig), nil
}

// SupplementOffline monta o infNFeSupl de uma NFC-e emitida em contingência offline.
func SupplementOffline(uf string, o Offline, signer Signer) (*nfe.InfNFeSupl, error) {
	qr, chave, err := URLs(uf, o.TpAmb)
	if err != nil {
		return nil, err
	}
	code, err := QRCodeV3Offline(qr, o, signer)
	if err != nil {
		return nil, err
	}
	return &nfe.InfNFeSupl{QRCode: code, URLChave: chave}, nil
}
