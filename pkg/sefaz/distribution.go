package sefaz

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// ServiceDFeDistribution é a distribuição de DF-e do Ambiente Nacional: as notas e os
// eventos de interesse do CNPJ (emitidas contra ele, ou em que ele aparece).
const ServiceDFeDistribution Service = "dfe_distribution"

// DistributionQuery é a consulta: pelo último NSU (o modo normal, em lotes de até 50), por
// um NSU específico, ou por chave de acesso (depois da ciência, para baixar o XML completo).
type DistributionQuery struct {
	Environment Environment
	UFCode      int    // cUFAutor: a UF do interessado
	CNPJ        string // o interessado (destinatário)
	LastNSU     int64
	NSU         int64  // consulta um NSU específico, quando > 0
	AccessKey   string // consulta por chave, quando informada
}

// DistributedDoc é um documento recebido: o XML já descompactado.
type DistributedDoc struct {
	NSU    int64
	Schema string // resNFe_v1.01.xsd, procNFe_v4.00.xsd, resEvento_v1.01.xsd, procEventoNFe_v1.00.xsd
	XML    []byte
}

// DistributionResult é o retDistDFeInt.
type DistributionResult struct {
	// CStat: 137 nenhum documento (espere uma hora antes de consultar de novo), 138
	// documentos localizados, 656 consumo indevido (bloqueio de uma hora).
	CStat   int
	Motive  string
	LastNSU int64
	MaxNSU  int64
	Docs    []DistributedDoc
}

// MaxDocSize é o teto de um documento descompactado: um XML de NF-e tem centenas de KB, e
// o teto barra a bomba de compressão.
const MaxDocSize = 8 << 20

// Distribute consulta a distribuição de DF-e. O envelope é o do serviço do Ambiente
// Nacional: nfeDistDFeInteresse > nfeDadosMsg, sem cabeçalho.
func (c *Client) Distribute(ctx context.Context, q DistributionQuery) (DistributionResult, error) {
	ep, err := c.table.Resolve(NFe, "AN", q.Environment, ServiceDFeDistribution, NoContingency)
	if err != nil {
		return DistributionResult{}, err
	}
	var body strings.Builder
	fmt.Fprintf(&body, `<distDFeInt xmlns="%s" versao="1.01"><tpAmb>%d</tpAmb><cUFAutor>%d</cUFAutor><CNPJ>%s</CNPJ>`,
		nfeNamespace, int(q.Environment), q.UFCode, xmlEscape(q.CNPJ))
	switch {
	case q.AccessKey != "":
		fmt.Fprintf(&body, `<consChNFe><chNFe>%s</chNFe></consChNFe>`, xmlEscape(q.AccessKey))
	case q.NSU > 0:
		fmt.Fprintf(&body, `<consNSU><NSU>%015d</NSU></consNSU>`, q.NSU)
	default:
		fmt.Fprintf(&body, `<distNSU><ultNSU>%015d</ultNSU></distNSU>`, q.LastNSU)
	}
	body.WriteString(`</distDFeInt>`)

	var env bytes.Buffer
	env.WriteString(`<?xml version="1.0" encoding="utf-8"?><soap12:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:soap12="http://www.w3.org/2003/05/soap-envelope"><soap12:Body>`)
	env.WriteString(`<nfeDistDFeInteresse xmlns="` + ep.Namespace() + `"><nfeDadosMsg xmlns="` + ep.Namespace() + `">`)
	env.WriteString(body.String())
	env.WriteString(`</nfeDadosMsg></nfeDistDFeInteresse></soap12:Body></soap12:Envelope>`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, &env)
	if err != nil {
		return DistributionResult{}, err
	}
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+ep.Action()+`"`)
	resp, err := c.http.Do(req)
	if err != nil {
		return DistributionResult{}, classifyTransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return DistributionResult{}, ErrCertificateRejected
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResp+1))
	if err != nil || int64(len(raw)) > c.maxResp {
		return DistributionResult{}, fmt.Errorf("%w: resposta ilegível ou grande demais", ErrUncertain)
	}
	var envelope struct {
		Ret *struct {
			CStat   int    `xml:"cStat"`
			Motive  string `xml:"xMotivo"`
			UltNSU  string `xml:"ultNSU"`
			MaxNSU  string `xml:"maxNSU"`
			DocZips []struct {
				NSU    string `xml:"NSU,attr"`
				Schema string `xml:"schema,attr"`
				Data   string `xml:",chardata"`
			} `xml:"loteDistDFeInt>docZip"`
		} `xml:"Body>nfeDistDFeInteresseResponse>nfeDistDFeInteresseResult>retDistDFeInt"`
	}
	if err := xml.Unmarshal(raw, &envelope); err != nil || envelope.Ret == nil {
		return DistributionResult{}, fmt.Errorf("%w: retDistDFeInt ilegível (HTTP %d)", ErrUncertain, resp.StatusCode)
	}
	r := envelope.Ret
	out := DistributionResult{CStat: r.CStat, Motive: r.Motive}
	out.LastNSU, _ = strconv.ParseInt(r.UltNSU, 10, 64)
	out.MaxNSU, _ = strconv.ParseInt(r.MaxNSU, 10, 64)
	for _, z := range r.DocZips {
		doc, err := gunzip(strings.TrimSpace(z.Data))
		if err != nil {
			return DistributionResult{}, fmt.Errorf("sefaz: docZip do NSU %s ilegível: %w", z.NSU, err)
		}
		nsu, _ := strconv.ParseInt(z.NSU, 10, 64)
		out.Docs = append(out.Docs, DistributedDoc{NSU: nsu, Schema: z.Schema, XML: doc})
	}
	return out, nil
}

func gunzip(b64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, MaxDocSize+1))
	if err != nil {
		return nil, err
	}
	if len(out) > MaxDocSize {
		return nil, fmt.Errorf("documento maior que %d bytes descompactado", MaxDocSize)
	}
	return out, nil
}
