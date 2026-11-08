// Package nfce monta o que a NFC-e (modelo 65) tem além da NF-e: o QR Code e a URL de
// consulta por chave (grupo infNFeSupl).
//
// O QR Code é o da versão 3 (NT 2025.001). Na emissão online ele é só
// URL?p=chave|3|tpAmb — sem o hash do CSC que a versão 2 exigia. A v3 é obrigatória para
// consumidor pessoa física e para contingência, e aceita para pessoa jurídica; a v2 fica de
// fora de propósito: ela exige guardar o CSC, e um segredo a menos é um vazamento a menos.
package nfce

import (
	_ "embed"
	"encoding/json"
	"errors"
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
