package sefaz

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const nfeNamespace = "http://www.portalfiscal.inf.br/nfe"

var (
	// ErrUncertain: a requisição pode ter chegado à SEFAZ sem a resposta chegar aqui
	// (timeout, conexão caída, resposta ilegível). Para autorização, isso NUNCA autoriza
	// reenviar: consulte pela chave primeiro (Constituição, Art. 6.1).
	ErrUncertain = errors.New("sefaz: sem resposta conclusiva — a SEFAZ pode ter recebido; consulte pela chave antes de reenviar")
	// ErrCertificateRejected: a SEFAZ recusou o certificado do cliente (HTTP 403) — vencido,
	// revogado, fora da ICP-Brasil, ou de um CNPJ sem credenciamento na UF. A requisição
	// não chegou ao serviço, então não há resultado incerto.
	ErrCertificateRejected = errors.New("sefaz: a SEFAZ recusou o certificado digital")
	// ErrServerUntrusted: o certificado do SERVIDOR não fechou na cadeia confiável. Nada foi
	// enviado. Endereço errado ou interceptação no caminho — nunca desligue a verificação.
	ErrServerUntrusted = errors.New("sefaz: o certificado do servidor não é confiável")
	// ErrSOAPFault: a SEFAZ respondeu com falha SOAP (envelope ou serviço errado).
	ErrSOAPFault = errors.New("sefaz: falha SOAP")
)

// Config monta o cliente.
type Config struct {
	// Certificate é o par do A1 para o mTLS (certificate.Certificate.TLS()).
	Certificate tls.Certificate
	// Table resolve os endereços; nil usa a embutida.
	Table *Table
	// Timeout por chamada. O padrão é 30 s: a autorização síncrona da SEFAZ responde em
	// poucos segundos quando está saudável, e esperar mais só prende a requisição.
	Timeout time.Duration
	// HTTPClient substitui o cliente HTTP (testes, proxy). Quando informado, Certificate
	// e a cadeia ICP-Brasil são responsabilidade de quem o montou.
	HTTPClient *http.Client
	// MaxResponseSize é o teto da resposta. O padrão é 8 MB.
	MaxResponseSize int64
	// RootCAs reaproveita uma cadeia já carregada (TrustPool). Montar a cadeia verifica
	// 175 certificados; quem cria um cliente por nota deve carregá-la uma vez só.
	RootCAs *x509.CertPool
}

// Client chama os web services.
type Client struct {
	http    *http.Client
	table   *Table
	maxResp int64
}

// NewClient monta o cliente com mTLS e a cadeia ICP-Brasil verificada.
func NewClient(cfg Config) (*Client, error) {
	table := cfg.Table
	if table == nil {
		var err error
		if table, err = DefaultTable(); err != nil {
			return nil, err
		}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxResp := cfg.MaxResponseSize
	if maxResp <= 0 {
		maxResp = 8 << 20
	}
	hc := cfg.HTTPClient
	if hc == nil {
		pool := cfg.RootCAs
		if pool == nil {
			var err error
			if pool, err = TrustPool(); err != nil {
				return nil, err
			}
		}
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{
			RootCAs:      pool,
			Certificates: []tls.Certificate{cfg.Certificate},
			MinVersion:   tls.VersionTLS12,
			// Algumas autorizadoras ainda renegociam a sessão para pedir o certificado.
			Renegotiation: tls.RenegotiateOnceAsClient,
			NextProtos:    []string{"http/1.1"},
		}
		// HTTP/1.1 sempre: a SEFAZ-SP recusa HTTP/2 com HTTP_1_1_REQUIRED (visto em
		// 2026-11-08), e a renegociação acima não existe em HTTP/2.
		tr.ForceAttemptHTTP2 = false
		tr.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		hc = &http.Client{Transport: tr, Timeout: timeout}
	}
	return &Client{http: hc, table: table, maxResp: maxResp}, nil
}

// Table devolve a tabela de endereços em uso.
func (c *Client) Table() *Table { return c.table }

// call envia o XML (já assinado, quando é o caso) dentro do envelope e devolve o conteúdo
// do nfeResultMsg.
func (c *Client) call(ctx context.Context, ep Endpoint, body []byte) ([]byte, error) {
	var env bytes.Buffer
	env.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	env.WriteString(`<soap12:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">`)
	env.WriteString(`<soap12:Body><nfeDadosMsg xmlns="` + ep.Namespace() + `">`)
	env.Write(body)
	env.WriteString(`</nfeDadosMsg></soap12:Body></soap12:Envelope>`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, &env)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+ep.Action()+`"`)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, classifyTransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrCertificateRejected
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResp+1))
	if err != nil || int64(len(raw)) > c.maxResp {
		return nil, fmt.Errorf("%w: resposta ilegível ou grande demais", ErrUncertain)
	}

	var envelope struct {
		Body struct {
			Result *struct {
				Inner []byte `xml:",innerxml"`
			} `xml:"nfeResultMsg"`
			Fault *struct {
				Reason string `xml:"Reason>Text"`
			} `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("%w: HTTP %d e corpo que não é SOAP", ErrUncertain, resp.StatusCode)
	}
	if f := envelope.Body.Fault; f != nil {
		return nil, fmt.Errorf("%w: %s", ErrSOAPFault, strings.TrimSpace(f.Reason))
	}
	if envelope.Body.Result == nil {
		return nil, fmt.Errorf("%w: HTTP %d sem nfeResultMsg", ErrUncertain, resp.StatusCode)
	}
	return bytes.TrimSpace(envelope.Body.Result.Inner), nil
}

// classifyTransportError separa o erro em que a SEFAZ certamente não processou nada do
// erro incerto.
//
// Alerta TLS da SEFAZ é recusa do certificado do cliente: servidor que recusa a
// autenticação não processa a requisição, mesmo no TLS 1.3, em que o cliente pode já ter
// mandado bytes. Falha de verificação do servidor acontece antes de qualquer envio. O
// resto — timeout, conexão caída — é incerto.
func classifyTransportError(err error) error {
	// Sobre TCP, o crypto/tls entrega o alerta recebido como net.OpError{Op: "remote
	// error"}; tls.AlertError só aparece no QUIC.
	var op *net.OpError
	var alert tls.AlertError
	if (errors.As(err, &op) && op.Op == "remote error") || errors.As(err, &alert) {
		return fmt.Errorf("%w: %v", ErrCertificateRejected, err)
	}
	var verr *tls.CertificateVerificationError
	if errors.As(err, &verr) {
		return fmt.Errorf("%w: %v", ErrServerUntrusted, err)
	}
	return fmt.Errorf("%w: %v", ErrUncertain, err)
}

// Target identifica para onde vai a chamada.
type Target struct {
	Model       Model
	UF          string // UF do emitente
	Environment Environment
	Contingency Contingency
}

// StatusResult é o retConsStatServ.
type StatusResult struct {
	CStat       int
	Motive      string
	AppVersion  string
	ReceivedAt  string
	AvgSeconds  int
	Observation string
	Raw         []byte
}

// Online diz se o serviço está em operação (cStat 107).
func (s StatusResult) Online() bool { return s.CStat == 107 }

// Status consulta o status do serviço — o primeiro teste de qualquer instalação: se ele
// responde, certificado, cadeia e endereço estão certos.
func (c *Client) Status(ctx context.Context, t Target) (StatusResult, error) {
	ep, err := c.table.Resolve(t.Model, t.UF, t.Environment, ServiceStatus, t.Contingency)
	if err != nil {
		return StatusResult{}, err
	}
	cuf := UFCode(t.UF)
	if cuf == 0 {
		return StatusResult{}, fmt.Errorf("sefaz: UF %q desconhecida", t.UF)
	}
	body := `<consStatServ xmlns="` + nfeNamespace + `" versao="4.00"><tpAmb>` + strconv.Itoa(int(t.Environment)) +
		`</tpAmb><cUF>` + strconv.Itoa(cuf) + `</cUF><xServ>STATUS</xServ></consStatServ>`
	raw, err := c.call(ctx, ep, []byte(body))
	if err != nil {
		return StatusResult{}, err
	}
	var r struct {
		CStat      int    `xml:"cStat"`
		Motive     string `xml:"xMotivo"`
		AppVersion string `xml:"verAplic"`
		ReceivedAt string `xml:"dhRecbto"`
		AvgSeconds int    `xml:"tMed"`
		Obs        string `xml:"xObs"`
	}
	if err := xml.Unmarshal(raw, &r); err != nil {
		return StatusResult{}, fmt.Errorf("sefaz: retConsStatServ ilegível: %w", err)
	}
	return StatusResult{CStat: r.CStat, Motive: r.Motive, AppVersion: r.AppVersion, ReceivedAt: r.ReceivedAt,
		AvgSeconds: r.AvgSeconds, Observation: r.Obs, Raw: raw}, nil
}

// Protocol é o infProt de uma nota.
type Protocol struct {
	CStat      int
	Motive     string
	Key        string
	Number     string // nProt
	ReceivedAt string
	DigestB64  string
	// Raw é o protNFe inteiro, como veio: é ele que entra no nfeProc de distribuição.
	Raw []byte
}

// Authorized diz se a nota está autorizada: 100, ou 150 (autorizada fora do prazo).
func (p Protocol) Authorized() bool { return p.CStat == 100 || p.CStat == 150 }

type protNFe struct {
	Inner []byte `xml:",innerxml"`
	Info  struct {
		CStat      int    `xml:"cStat"`
		Motive     string `xml:"xMotivo"`
		Key        string `xml:"chNFe"`
		Number     string `xml:"nProt"`
		ReceivedAt string `xml:"dhRecbto"`
		Digest     string `xml:"digVal"`
	} `xml:"infProt"`
}

func (p *protNFe) toProtocol() *Protocol {
	if p == nil {
		return nil
	}
	raw := []byte(`<protNFe xmlns="` + nfeNamespace + `" versao="4.00">` + string(p.Inner) + `</protNFe>`)
	return &Protocol{CStat: p.Info.CStat, Motive: p.Info.Motive, Key: p.Info.Key, Number: p.Info.Number,
		ReceivedAt: p.Info.ReceivedAt, DigestB64: p.Info.Digest, Raw: raw}
}

// QueryResult é o retConsSitNFe.
type QueryResult struct {
	CStat    int
	Motive   string
	Protocol *Protocol // nil quando a SEFAZ não conhece a chave
	Raw      []byte
}

// Query consulta a nota pela chave. É a chamada que decide o que fazer depois de um
// ErrUncertain: cStat 100/150 (autorizada), 110/301/302 (denegada) ou 217 (a SEFAZ não
// conhece a chave — só então reenviar).
func (c *Client) Query(ctx context.Context, t Target, key string) (QueryResult, error) {
	ep, err := c.table.Resolve(t.Model, t.UF, t.Environment, ServiceProtocolQuery, t.Contingency)
	if err != nil {
		return QueryResult{}, err
	}
	body := `<consSitNFe xmlns="` + nfeNamespace + `" versao="4.00"><tpAmb>` + strconv.Itoa(int(t.Environment)) +
		`</tpAmb><xServ>CONSULTAR</xServ><chNFe>` + xmlEscape(key) + `</chNFe></consSitNFe>`
	raw, err := c.call(ctx, ep, []byte(body))
	if err != nil {
		return QueryResult{}, err
	}
	var r struct {
		CStat  int      `xml:"cStat"`
		Motive string   `xml:"xMotivo"`
		Prot   *protNFe `xml:"protNFe"`
	}
	if err := xml.Unmarshal(raw, &r); err != nil {
		return QueryResult{}, fmt.Errorf("%w: retConsSitNFe ilegível", ErrUncertain)
	}
	return QueryResult{CStat: r.CStat, Motive: r.Motive, Protocol: r.Prot.toProtocol(), Raw: raw}, nil
}

// AuthorizeResult é o retEnviNFe.
type AuthorizeResult struct {
	// BatchCStat é o cStat do lote: 104 = processado (o resultado da nota está em
	// Protocol); outro valor é rejeição do lote inteiro.
	BatchCStat  int
	BatchMotive string
	Protocol    *Protocol
	Raw         []byte
}

// Authorize envia UMA nota já assinada em modo síncrono (indSinc = 1).
//
// Um lote de uma nota é deliberado: com várias, uma rejeitada atrasa todas, e o resultado
// de cada uma fica amarrado ao recibo do lote. A SEFAZ aceita até 50, mas o ganho não paga
// a complexidade — e o modo síncrono só vale para lote de uma nota.
func (c *Client) Authorize(ctx context.Context, t Target, batchID int64, signedNFe []byte) (AuthorizeResult, error) {
	ep, err := c.table.Resolve(t.Model, t.UF, t.Environment, ServiceAuthorization, t.Contingency)
	if err != nil {
		return AuthorizeResult{}, err
	}
	doc := bytes.TrimSpace(signedNFe)
	if i := bytes.Index(doc, []byte("?>")); bytes.HasPrefix(doc, []byte("<?xml")) && i > 0 {
		doc = bytes.TrimSpace(doc[i+2:])
	}
	var body bytes.Buffer
	body.WriteString(`<enviNFe xmlns="` + nfeNamespace + `" versao="4.00"><idLote>` + strconv.FormatInt(batchID, 10) +
		`</idLote><indSinc>1</indSinc>`)
	body.Write(doc)
	body.WriteString(`</enviNFe>`)
	raw, err := c.call(ctx, ep, body.Bytes())
	if err != nil {
		return AuthorizeResult{}, err
	}
	var r struct {
		CStat  int      `xml:"cStat"`
		Motive string   `xml:"xMotivo"`
		Prot   *protNFe `xml:"protNFe"`
	}
	if err := xml.Unmarshal(raw, &r); err != nil {
		return AuthorizeResult{}, fmt.Errorf("%w: retEnviNFe ilegível", ErrUncertain)
	}
	return AuthorizeResult{BatchCStat: r.CStat, BatchMotive: r.Motive, Protocol: r.Prot.toProtocol(), Raw: raw}, nil
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func xmlEscape(s string) string { return xmlEscaper.Replace(s) }

// EventResult é o retEnvEvento.
type EventResult struct {
	// BatchCStat é o do lote: 128 = processado (o resultado do evento está em CStat).
	BatchCStat  int
	BatchMotive string
	// CStat do evento: 135 registrado e vinculado à nota, 136 registrado sem vínculo, 155
	// cancelamento homologado fora do prazo; o resto é rejeição.
	CStat        int
	Motive       string
	Protocol     string
	RegisteredAt string
	// RetEvento é o retEvento inteiro, como veio: entra no procEventoNFe.
	RetEvento []byte
	Raw       []byte
}

// Registered diz se o evento foi registrado.
func (e EventResult) Registered() bool { return e.CStat == 135 || e.CStat == 136 || e.CStat == 155 }

// SendEvent envia um evento já assinado (cancelamento, carta de correção).
func (c *Client) SendEvent(ctx context.Context, t Target, batchID int64, signedEvento []byte) (EventResult, error) {
	ep, err := c.table.Resolve(t.Model, t.UF, t.Environment, ServiceEvent, t.Contingency)
	if err != nil {
		return EventResult{}, err
	}
	doc := bytes.TrimSpace(signedEvento)
	if i := bytes.Index(doc, []byte("?>")); bytes.HasPrefix(doc, []byte("<?xml")) && i > 0 {
		doc = bytes.TrimSpace(doc[i+2:])
	}
	var body bytes.Buffer
	body.WriteString(`<envEvento xmlns="` + nfeNamespace + `" versao="1.00"><idLote>` + strconv.FormatInt(batchID, 10) + `</idLote>`)
	body.Write(doc)
	body.WriteString(`</envEvento>`)
	raw, err := c.call(ctx, ep, body.Bytes())
	if err != nil {
		return EventResult{}, err
	}
	var r struct {
		CStat  int    `xml:"cStat"`
		Motive string `xml:"xMotivo"`
		Ret    *struct {
			Inner []byte `xml:",innerxml"`
			Info  struct {
				CStat        int    `xml:"cStat"`
				Motive       string `xml:"xMotivo"`
				Protocol     string `xml:"nProt"`
				RegisteredAt string `xml:"dhRegEvento"`
			} `xml:"infEvento"`
		} `xml:"retEvento"`
	}
	if err := xml.Unmarshal(raw, &r); err != nil {
		return EventResult{}, fmt.Errorf("%w: retEnvEvento ilegível", ErrUncertain)
	}
	out := EventResult{BatchCStat: r.CStat, BatchMotive: r.Motive, Raw: raw}
	if r.Ret != nil {
		i := r.Ret.Info
		out.CStat, out.Motive, out.Protocol, out.RegisteredAt = i.CStat, i.Motive, i.Protocol, i.RegisteredAt
		out.RetEvento = []byte(`<retEvento xmlns="` + nfeNamespace + `" versao="1.00">` + string(r.Ret.Inner) + `</retEvento>`)
	}
	return out, nil
}
