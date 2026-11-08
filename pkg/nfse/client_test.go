package nfse_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/nfse"
)

const key = "35503082211222333000181000000000004226115678901234"

func fakeSefin(t *testing.T, handler http.HandlerFunc) *nfse.Client {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	c, err := nfse.NewClient(nfse.Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEmissaoNaSefin(t *testing.T) {
	nfseXML := `<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.01"><infNFSe Id="NFS` + key + `"><nNFSe>42</nNFSe></infNFSe></NFSe>`
	packed, _ := nfse.Pack([]byte(nfseXML))
	c := fakeSefin(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || r.URL.Path != "/nfse" || !strings.Contains(string(body), `"dpsXmlGZipB64"`) {
			t.Errorf("pedido: %s %s %s", r.Method, r.URL.Path, body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"chaveAcesso":"`+key+`","nfseXmlGZipB64":"`+packed+`","alertas":[{"Codigo":"A0001","Descricao":"alerta de teste"}]}`)
	})
	res, err := c.Issue(context.Background(), []byte(`<DPS/>`))
	if err != nil || !res.Authorized() || res.AccessKey != key || res.Number != "42" || len(res.Alerts) != 1 {
		t.Errorf("emissão: %+v %v", res, err)
	}
}

func TestEmissaoRecusadaComCodigo(t *testing.T) {
	c := fakeSefin(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"Erros":[{"Codigo":"E0312","Descricao":"Codigo de tributacao nacional inexistente","Complemento":"cTribNac 999999"}]}`)
	})
	res, err := c.Issue(context.Background(), []byte(`<DPS/>`))
	if err != nil || res.Authorized() || len(res.Errors) != 1 || res.Errors[0].Code != "E0312" {
		t.Errorf("recusa: %+v %v", res, err)
	}
}

func TestConsultaDPSEIncerteza(t *testing.T) {
	c := fakeSefin(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dps/DPS-SIM":
			_, _ = io.WriteString(w, `{"chaveAcesso":"`+key+`"}`)
		case "/dps/DPS-NAO":
			w.WriteHeader(http.StatusNotFound)
		case "/nfse":
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		case "/dps/DPS-CERT":
			w.WriteHeader(http.StatusForbidden)
		}
	})
	if k, found, err := c.DPSKey(context.Background(), "DPS-SIM"); err != nil || !found || k != key {
		t.Errorf("DPS emitida: %q %v %v", k, found, err)
	}
	if _, found, err := c.DPSKey(context.Background(), "DPS-NAO"); err != nil || found {
		t.Errorf("DPS não emitida: %v %v", found, err)
	}
	if _, err := c.Issue(context.Background(), []byte(`<DPS/>`)); !errors.Is(err, nfse.ErrUncertain) {
		t.Errorf("queda de conexão: %v", err)
	}
	if _, _, err := c.DPSKey(context.Background(), "DPS-CERT"); !errors.Is(err, nfse.ErrCertificateRejected) {
		t.Errorf("403: %v", err)
	}
}
