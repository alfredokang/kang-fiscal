package nfse_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/nfse"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

func signedEvent(t *testing.T, c nfse.Cancellation) []byte {
	t.Helper()
	p, err := nfse.NewCancellation(c)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := nfse.MarshalEvent(p)
	if err != nil {
		t.Fatal(err)
	}
	pfx, _ := certificate.TestPFX("CAFE EXEMPLO LTDA", "11222333000181", "s", time.Now().Add(time.Hour))
	cert, _ := certificate.Load(pfx, "s")
	out, err := xmldsig.Sign(raw, "infPedReg", cert)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xmldsig.Verify(out); err != nil {
		t.Fatal(err)
	}
	return out
}

func validateEvent(t *testing.T, doc []byte) {
	t.Helper()
	bin, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("xmllint não instalado")
	}
	f := filepath.Join(t.TempDir(), "evento.xml")
	_ = os.WriteFile(f, doc, 0o600)
	if out, err := exec.Command(bin, "--noout", "--schema", "testdata/schemas/20261020/pedRegEvento_v1.01.xsd", f).CombinedOutput(); err != nil {
		t.Fatalf("o pedido de evento não passa no XSD:\n%s\n%s", out, doc)
	}
}

func cancellation(analysis bool) nfse.Cancellation {
	return nfse.Cancellation{AccessKey: key, Author: "11222333000181", Reason: nfse.ReasonIssueError,
		Justification: "Valor do servico informado errado", Environment: 2, Analysis: analysis,
		At: time.Date(2026, 11, 8, 10, 0, 0, 0, time.FixedZone("-03", -3*3600))}
}

func TestCancelamentoPassaNoXSD(t *testing.T) {
	doc := signedEvent(t, cancellation(false))
	if !strings.Contains(string(doc), `Id="PRE`+key+`101101"`) {
		t.Errorf("Id do pedido: %s", doc)
	}
	validateEvent(t, doc)
}

func TestPedidoDeAnaliseFiscalPassaNoXSD(t *testing.T) {
	doc := signedEvent(t, cancellation(true))
	if !strings.Contains(string(doc), "<xDesc>Solicitação de Análise Fiscal para Cancelamento de NFS-e</xDesc>") {
		t.Errorf("o xDesc precisa ir acentuado: %s", doc)
	}
	validateEvent(t, doc)
}

func TestCancelamentoRecusaForaDoLeiaute(t *testing.T) {
	for name, mut := range map[string]func(*nfse.Cancellation){
		"justificativa curta": func(c *nfse.Cancellation) { c.Justification = "erro" },
		"motivo 3":            func(c *nfse.Cancellation) { c.Reason = 3 },
		"chave curta":         func(c *nfse.Cancellation) { c.AccessKey = "123" },
		"autor":               func(c *nfse.Cancellation) { c.Author = "123" },
	} {
		c := cancellation(false)
		mut(&c)
		if _, err := nfse.NewCancellation(c); !errors.Is(err, nfse.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEnvioDoEvento(t *testing.T) {
	ev := `<evento xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.01"><infEvento Id="EVT` + key + `101101001"/></evento>`
	packed, _ := nfse.Pack([]byte(ev))
	c := fakeSefin(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/nfse/"+key+"/eventos" || !strings.Contains(string(body), `"pedidoRegistroEventoXmlGZipB64"`) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"tipoAmbiente":2,"eventoXmlGZipB64":"` + packed + `"}`))
	})
	res, err := c.SendEvent(context.Background(), key, []byte("<pedRegEvento/>"))
	if err != nil || !res.Registered() || !strings.Contains(string(res.EventXML), "EVT"+key) {
		t.Fatalf("evento: %v %+v", err, res)
	}

	c = fakeSefin(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"erro":{"codigo":"E0840","descricao":"Já existe evento de cancelamento vinculado"}}`))
	})
	if res, err := c.SendEvent(context.Background(), key, []byte("<pedRegEvento/>")); err != nil || !res.AlreadyLinked() || res.Registered() {
		t.Errorf("E0840: %v %+v", err, res)
	}

	c = fakeSefin(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	if _, err := c.SendEvent(context.Background(), key, []byte("<pedRegEvento/>")); !errors.Is(err, nfse.ErrUncertain) {
		t.Errorf("502 sem corpo é incerto: %v", err)
	}
}

func TestEstadoDoCancelamento(t *testing.T) {
	has := map[string]bool{}
	c := fakeSefin(t, func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if has[parts[len(parts)-2]] {
			_, _ = w.Write([]byte(`{"eventos":[{"tipo":1}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"erro":{"codigo":"E404"}}`))
	})
	ctx := context.Background()
	if s, err := c.CancelState(ctx, key); err != nil || s != nfse.CancelStateNone {
		t.Errorf("sem eventos: %s %v", s, err)
	}
	has[nfse.EventCancelAnalysis] = true
	if s, _ := c.CancelState(ctx, key); s != nfse.CancelStateAnalysis {
		t.Errorf("em análise: %s", s)
	}
	has[nfse.EventAnalysisGranted] = true
	if s, _ := c.CancelState(ctx, key); s != nfse.CancelStateCanceled {
		t.Errorf("deferido: %s", s)
	}

	c = fakeSefin(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	if _, err := c.CancelState(ctx, key); err == nil {
		t.Error("404 sem JSON é rota errada, não 'sem evento'")
	}
}
