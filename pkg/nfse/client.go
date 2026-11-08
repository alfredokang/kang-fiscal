package nfse

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
)

// Endereços da Sefin Nacional.
const (
	ProductionURL   = "https://sefin.nfse.gov.br/SefinNacional"
	HomologationURL = "https://sefin.producaorestrita.nfse.gov.br/SefinNacional"
)

var (
	// ErrUncertain: a DPS pode ter chegado sem a resposta chegar aqui. Consulte a DPS pelo
	// Id (DPSKey) antes de reenviar.
	ErrUncertain = errors.New("nfse: sem resposta conclusiva do Sistema Nacional — consulte a DPS antes de reenviar")
	// ErrCertificateRejected: o Sistema Nacional recusou o certificado (401/403). Nada foi
	// processado.
	ErrCertificateRejected = errors.New("nfse: o Sistema Nacional recusou o certificado digital")
)

// Config monta o cliente.
type Config struct {
	Certificate tls.Certificate
	Environment int    // 1 produção, 2 homologação (produção restrita)
	BaseURL     string // sobrepõe o endereço (município com Sefin própria, testes)
	HTTPClient  *http.Client
	RootCAs     *x509.CertPool
	Timeout     time.Duration
}

// Client fala com a API REST da Sefin.
type Client struct {
	base string
	http *http.Client
}

// NewClient monta o cliente com mTLS e a cadeia ICP-Brasil verificada (a mesma da NF-e).
func NewClient(cfg Config) (*Client, error) {
	base := cfg.BaseURL
	if base == "" {
		base = HomologationURL
		if cfg.Environment == 1 {
			base = ProductionURL
		}
	}
	hc := cfg.HTTPClient
	if hc == nil {
		pool := cfg.RootCAs
		if pool == nil {
			var err error
			if pool, err = sefaz.TrustPool(); err != nil {
				return nil, err
			}
		}
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 40 * time.Second
		}
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{RootCAs: pool, Certificates: []tls.Certificate{cfg.Certificate}, MinVersion: tls.VersionTLS12}
		hc = &http.Client{Transport: tr, Timeout: timeout}
	}
	return &Client{base: strings.TrimRight(base, "/"), http: hc}, nil
}

// Message é um erro ou alerta do Sistema Nacional (E-codes como E0312).
type Message struct {
	Code        string
	Description string
	Complement  string
}

// IssueResult é a resposta da emissão.
type IssueResult struct {
	HTTPStatus int
	AccessKey  string // 50 posições
	Number     string // nNFSe
	NFSeXML    []byte
	Errors     []Message
	Alerts     []string
}

// Authorized diz se a NFS-e foi gerada.
func (r IssueResult) Authorized() bool { return len(r.NFSeXML) > 0 && len(r.Errors) == 0 }

// Issue envia a DPS assinada: POST /nfse com {"dpsXmlGZipB64": ...}.
func (c *Client) Issue(ctx context.Context, signedDPS []byte) (IssueResult, error) {
	packed, err := Pack(signedDPS)
	if err != nil {
		return IssueResult{}, err
	}
	body, _ := json.Marshal(map[string]string{"dpsXmlGZipB64": packed})
	resp, raw, err := c.do(ctx, http.MethodPost, "/nfse", body)
	if err != nil {
		return IssueResult{}, err
	}
	res := IssueResult{HTTPStatus: resp.StatusCode}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return res, fmt.Errorf("%w: HTTP %d sem JSON", ErrUncertain, resp.StatusCode)
	}
	res.Errors = messages(data, "erros", "erro")
	for _, a := range messages(data, "alertas") {
		res.Alerts = append(res.Alerts, strings.TrimSpace(a.Code+" "+a.Description))
	}
	if len(res.Errors) > 0 {
		return res, nil
	}
	b64, _ := field(data, "nfseXmlGZipB64").(string)
	if b64 == "" {
		return res, fmt.Errorf("%w: HTTP %d sem a NFS-e", ErrUncertain, resp.StatusCode)
	}
	if res.NFSeXML, err = Unpack(b64); err != nil {
		return res, fmt.Errorf("%w: NFS-e ilegível", ErrUncertain)
	}
	res.AccessKey, _ = field(data, "chaveAcesso").(string)
	if res.AccessKey == "" {
		res.AccessKey = KeyFromXML(res.NFSeXML)
	}
	res.Number = tag(res.NFSeXML, "nNFSe")
	return res, nil
}

// DPSKey consulta se a DPS já gerou NFS-e: GET /dps/{id}. É o que decide o que fazer
// depois de um ErrUncertain — o equivalente da consulta pela chave da NF-e.
func (c *Client) DPSKey(ctx context.Context, dpsID string) (key string, found bool, err error) {
	resp, raw, err := c.do(ctx, http.MethodGet, "/dps/"+dpsID, nil)
	if err != nil {
		return "", false, err
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return "", false, nil
	case http.StatusOK:
		var data map[string]any
		_ = json.Unmarshal(raw, &data)
		k, _ := field(data, "chaveAcesso").(string)
		return k, k != "", nil
	}
	return "", false, fmt.Errorf("%w: consulta da DPS HTTP %d", ErrUncertain, resp.StatusCode)
}

// Get baixa a NFS-e pela chave: GET /nfse/{chave}.
func (c *Client) Get(ctx context.Context, key string) ([]byte, bool, error) {
	resp, raw, err := c.do(ctx, http.MethodGet, "/nfse/"+key, nil)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	var data map[string]any
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &data) != nil {
		return nil, false, fmt.Errorf("%w: consulta da NFS-e HTTP %d", ErrUncertain, resp.StatusCode)
	}
	b64, _ := field(data, "nfseXmlGZipB64").(string)
	doc, err := Unpack(b64)
	return doc, err == nil, err
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) (*http.Response, []byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		var op *net.OpError
		if errors.As(err, &op) && op.Op == "remote error" {
			return nil, nil, fmt.Errorf("%w: %v", ErrCertificateRejected, err)
		}
		return nil, nil, fmt.Errorf("%w: %v", ErrUncertain, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, nil, ErrCertificateRejected
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrUncertain, err)
	}
	return resp, raw, nil
}

// Pack é o gzip+base64 que a API exige.
func Pack(doc []byte) (string, error) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write(doc); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

// Unpack desfaz o gzip+base64, com teto contra bomba de compressão.
func Unpack(b64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, err
	}
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, 8<<20+1))
	if err != nil {
		return nil, err
	}
	if len(out) > 8<<20 {
		return nil, errors.New("nfse: documento grande demais")
	}
	return out, nil
}

var keyRe = regexp.MustCompile(`<infNFSe\b[^>]*\bId="NFS([0-9A-Z]{50})"`)

// KeyFromXML tira a chave do Id do infNFSe: o XML da NFS-e não traz chNFSe.
func KeyFromXML(doc []byte) string {
	if m := keyRe.FindSubmatch(doc); m != nil {
		return string(m[1])
	}
	return ""
}

func tag(doc []byte, name string) string {
	re := regexp.MustCompile(`<` + name + `>([^<]*)</` + name + `>`)
	if m := re.FindSubmatch(doc); m != nil {
		return string(m[1])
	}
	return ""
}

// field lê uma chave do JSON sem diferenciar maiúscula: a API devolve "erros" e "Erros",
// "codigo" e "Codigo", conforme o endpoint.
func field(m map[string]any, name string) any {
	for k, v := range m {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return nil
}

func messages(data map[string]any, names ...string) []Message {
	var out []Message
	for _, n := range names {
		switch v := field(data, n).(type) {
		case []any:
			for _, it := range v {
				if m, ok := it.(map[string]any); ok {
					out = append(out, toMessage(m))
				}
			}
		case map[string]any:
			out = append(out, toMessage(v))
		}
	}
	return out
}

func toMessage(m map[string]any) Message {
	s := func(name string, alts ...string) string {
		for _, n := range append([]string{name}, alts...) {
			if v, ok := field(m, n).(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	return Message{Code: s("codigo"), Description: s("descricao", "mensagem"), Complement: s("complemento")}
}
