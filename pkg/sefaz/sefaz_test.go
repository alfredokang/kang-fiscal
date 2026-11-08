package sefaz_test

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
)

func TestResolveEnderecos(t *testing.T) {
	table, err := sefaz.DefaultTable()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		model sefaz.Model
		uf    string
		env   sefaz.Environment
		svc   sefaz.Service
		cont  sefaz.Contingency
		want  string
	}{
		{sefaz.NFe, "SP", sefaz.Homologation, sefaz.ServiceStatus, "", "https://homologacao.nfe.fazenda.sp.gov.br/ws/nfestatusservico4.asmx"},
		// O RS tem autorizador próprio — não é a SVRS (a tabela do gonfe erra isso).
		{sefaz.NFe, "RS", sefaz.Homologation, sefaz.ServiceStatus, "", "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx"},
		{sefaz.NFe, "AC", sefaz.Homologation, sefaz.ServiceStatus, "", "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx"},
		{sefaz.NFe, "MA", sefaz.Production, sefaz.ServiceStatus, "", "https://www.sefazvirtual.fazenda.gov.br/NFeStatusServico4/NFeStatusServico4.asmx"},
		{sefaz.NFe, "SP", sefaz.Homologation, sefaz.ServiceAuthorization, sefaz.ContingencyFor("SP"), "https://hom.sefazvirtual.fazenda.gov.br/NFeAutorizacao4/NFeAutorizacao4.asmx"},
	}
	for _, c := range cases {
		ep, err := table.Resolve(c.model, c.uf, c.env, c.svc, c.cont)
		if err != nil || ep.URL != c.want {
			t.Errorf("%d %s %s: %q %v, esperava %q", c.model, c.uf, c.svc, ep.URL, err, c.want)
		}
	}
	// Toda UF tem os serviços essenciais nos dois modelos e nos dois ambientes.
	for _, uf := range []string{"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA", "MG", "MS", "MT", "PA",
		"PB", "PE", "PI", "PR", "RJ", "RN", "RO", "RR", "RS", "SC", "SE", "SP", "TO"} {
		for _, m := range []sefaz.Model{sefaz.NFe, sefaz.NFCe} {
			for _, env := range []sefaz.Environment{sefaz.Homologation, sefaz.Production} {
				for _, svc := range []sefaz.Service{sefaz.ServiceStatus, sefaz.ServiceAuthorization, sefaz.ServiceProtocolQuery, sefaz.ServiceEvent} {
					if _, err := table.Resolve(m, uf, env, svc, ""); err != nil {
						t.Errorf("faltando: %v", err)
					}
				}
			}
		}
		if sefaz.ContingencyFor(uf) == "" {
			t.Errorf("%s sem SVC", uf)
		}
	}
}

// fakeSEFAZ responde como uma autorizadora e confere o envelope recebido.
func fakeSEFAZ(t *testing.T, wantAction, reply string) (*httptest.Server, *sefaz.Client) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/soap+xml; charset=utf-8") || !strings.Contains(ct, `action="`+wantAction+`"`) {
			t.Errorf("Content-Type: %s", ct)
		}
		if !strings.Contains(string(body), "<soap12:Body><nfeDadosMsg xmlns=") {
			t.Errorf("envelope: %s", body)
		}
		w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?><soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><nfeResultMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/x">`+reply+`</nfeResultMsg></soap:Body></soap:Envelope>`)
	}))
	t.Cleanup(srv.Close)
	table, _ := sefaz.DefaultTable()
	for _, svc := range []sefaz.Service{sefaz.ServiceStatus, sefaz.ServiceProtocolQuery, sefaz.ServiceAuthorization} {
		op := map[sefaz.Service][2]string{
			sefaz.ServiceStatus:        {"NFeStatusServico4", "nfeStatusServicoNF"},
			sefaz.ServiceProtocolQuery: {"NFeConsultaProtocolo4", "nfeConsultaNF"},
			sefaz.ServiceAuthorization: {"NFeAutorizacao4", "nfeAutorizacaoLote"},
		}[svc]
		table.Override(sefaz.NFe, "SP", sefaz.Homologation, svc, sefaz.Endpoint{URL: srv.URL, Operation: op[0], Method: op[1], Version: "4.00"})
	}
	client, err := sefaz.NewClient(sefaz.Config{Table: table, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return srv, client
}

var sp = sefaz.Target{Model: sefaz.NFe, UF: "SP", Environment: sefaz.Homologation}

func TestStatus(t *testing.T) {
	_, c := fakeSEFAZ(t, "http://www.portalfiscal.inf.br/nfe/wsdl/NFeStatusServico4/nfeStatusServicoNF",
		`<retConsStatServ versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe"><tpAmb>2</tpAmb><verAplic>SP_NFE_PL009_V4</verAplic><cStat>107</cStat><xMotivo>Serviço em Operação</xMotivo><cUF>35</cUF><dhRecbto>2026-11-08T10:00:00-03:00</dhRecbto><tMed>1</tMed></retConsStatServ>`)
	st, err := c.Status(context.Background(), sp)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Online() || st.Motive != "Serviço em Operação" || st.AvgSeconds != 1 {
		t.Errorf("status: %+v", st)
	}
}

func TestAutorizacaoSincrona(t *testing.T) {
	_, c := fakeSEFAZ(t, "http://www.portalfiscal.inf.br/nfe/wsdl/NFeAutorizacao4/nfeAutorizacaoLote",
		`<retEnviNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe"><tpAmb>2</tpAmb><cStat>104</cStat><xMotivo>Lote processado</xMotivo><protNFe versao="4.00"><infProt><tpAmb>2</tpAmb><chNFe>35261111222333000181550010000001231123456787</chNFe><dhRecbto>2026-11-08T10:00:01-03:00</dhRecbto><nProt>135260000000001</nProt><digVal>abc=</digVal><cStat>100</cStat><xMotivo>Autorizado o uso da NF-e</xMotivo></infProt></protNFe></retEnviNFe>`)
	res, err := c.Authorize(context.Background(), sp, 1, []byte(`<?xml version="1.0"?><NFe xmlns="http://www.portalfiscal.inf.br/nfe"></NFe>`))
	if err != nil {
		t.Fatal(err)
	}
	p := res.Protocol
	if res.BatchCStat != 104 || p == nil || !p.Authorized() || p.Number != "135260000000001" {
		t.Fatalf("resultado: %+v %+v", res, p)
	}
	if !strings.HasPrefix(string(p.Raw), `<protNFe xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><infProt>`) {
		t.Errorf("protNFe para o nfeProc: %s", p.Raw)
	}
}

func TestConsultaChaveDesconhecida(t *testing.T) {
	_, c := fakeSEFAZ(t, "http://www.portalfiscal.inf.br/nfe/wsdl/NFeConsultaProtocolo4/nfeConsultaNF",
		`<retConsSitNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe"><tpAmb>2</tpAmb><cStat>217</cStat><xMotivo>Rejeição: NF-e não consta na base de dados da SEFAZ</xMotivo></retConsSitNFe>`)
	res, err := c.Query(context.Background(), sp, "35261111222333000181550010000001231123456787")
	if err != nil || res.CStat != 217 || res.Protocol != nil {
		t.Fatalf("consulta: %+v %v", res, err)
	}
}

// Conexão caída depois do envio é a situação do Art. 6.1: o erro precisa dizer que o
// resultado é incerto, para ninguém reenviar sem consultar.
func TestQuedaDeConexaoEIncerta(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()
	table, _ := sefaz.DefaultTable()
	table.Override(sefaz.NFe, "SP", sefaz.Homologation, sefaz.ServiceAuthorization, sefaz.Endpoint{URL: srv.URL, Operation: "NFeAutorizacao4", Method: "nfeAutorizacaoLote"})
	c, _ := sefaz.NewClient(sefaz.Config{Table: table, HTTPClient: srv.Client()})
	if _, err := c.Authorize(context.Background(), sp, 1, []byte(`<NFe/>`)); !errors.Is(err, sefaz.ErrUncertain) {
		t.Errorf("queda de conexão: %v", err)
	}
}

func TestFalhaSOAP(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><soap:Fault><soap:Reason><soap:Text>Server was unable to process request</soap:Text></soap:Reason></soap:Fault></soap:Body></soap:Envelope>`)
	}))
	defer srv.Close()
	table, _ := sefaz.DefaultTable()
	table.Override(sefaz.NFe, "SP", sefaz.Homologation, sefaz.ServiceStatus, sefaz.Endpoint{URL: srv.URL, Operation: "NFeStatusServico4", Method: "nfeStatusServicoNF"})
	c2, _ := sefaz.NewClient(sefaz.Config{Table: table, HTTPClient: srv.Client()})
	if _, err := c2.Status(context.Background(), sp); !errors.Is(err, sefaz.ErrSOAPFault) {
		t.Errorf("fault: %v", err)
	}
}

func TestCatalogoDeCodigos(t *testing.T) {
	c, ok := sefaz.Describe(778)
	if !ok || c.Message == "" || c.Hint == "" {
		t.Errorf("778: %+v", c)
	}
	if _, ok := sefaz.Describe(99999); ok {
		t.Error("código inexistente")
	}
}

func TestCertificadoRecusado(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	table, _ := sefaz.DefaultTable()
	table.Override(sefaz.NFe, "SP", sefaz.Homologation, sefaz.ServiceStatus, sefaz.Endpoint{URL: srv.URL, Operation: "NFeStatusServico4", Method: "nfeStatusServicoNF"})
	c, _ := sefaz.NewClient(sefaz.Config{Table: table, HTTPClient: srv.Client()})
	if _, err := c.Status(context.Background(), sp); !errors.Is(err, sefaz.ErrCertificateRejected) || errors.Is(err, sefaz.ErrUncertain) {
		t.Errorf("403: %v", err)
	}
}

// A SEFAZ que exige certificado de cliente recusa no handshake (MG, PR, AM, MT fazem
// assim). Não é resultado incerto: nada foi processado.
func TestRecusaNoHandshakeNaoEIncerta(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, MaxVersion: tls.VersionTLS12}
	srv.StartTLS()
	defer srv.Close()
	table, _ := sefaz.DefaultTable()
	table.Override(sefaz.NFe, "SP", sefaz.Homologation, sefaz.ServiceAuthorization, sefaz.Endpoint{URL: srv.URL, Operation: "NFeAutorizacao4", Method: "nfeAutorizacaoLote"})
	c, _ := sefaz.NewClient(sefaz.Config{Table: table, HTTPClient: srv.Client()})
	_, err := c.Authorize(context.Background(), sp, 1, []byte(`<NFe/>`))
	if !errors.Is(err, sefaz.ErrCertificateRejected) || errors.Is(err, sefaz.ErrUncertain) {
		t.Errorf("recusa no handshake: %v", err)
	}
}

func TestEventoRegistrado(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `<envEvento xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.00"><idLote>7</idLote><evento`) {
			t.Errorf("lote: %s", body)
		}
		_, _ = io.WriteString(w, `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><nfeResultMsg><retEnvEvento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe"><idLote>7</idLote><tpAmb>2</tpAmb><cStat>128</cStat><xMotivo>Lote de Evento Processado</xMotivo><retEvento versao="1.00"><infEvento><tpAmb>2</tpAmb><cStat>135</cStat><xMotivo>Evento registrado e vinculado a NF-e</xMotivo><nProt>135260000000099</nProt><dhRegEvento>2026-11-08T11:00:01-03:00</dhRegEvento></infEvento></retEvento></retEnvEvento></nfeResultMsg></soap:Body></soap:Envelope>`)
	}))
	defer srv.Close()
	table, _ := sefaz.DefaultTable()
	table.Override(sefaz.NFe, "SP", sefaz.Homologation, sefaz.ServiceEvent, sefaz.Endpoint{URL: srv.URL, Operation: "NFeRecepcaoEvento4", Method: "nfeRecepcaoEvento"})
	c, _ := sefaz.NewClient(sefaz.Config{Table: table, HTTPClient: srv.Client()})
	res, err := c.SendEvent(context.Background(), sp, 7, []byte(`<evento versao="1.00"></evento>`))
	if err != nil || !res.Registered() || res.Protocol != "135260000000099" || !strings.HasPrefix(string(res.RetEvento), "<retEvento") {
		t.Errorf("evento: %+v %v", res, err)
	}
}
