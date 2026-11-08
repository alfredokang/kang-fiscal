package server_test

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

// offlineSefaz: a SEFAZ não responde às NFC-e online; responde às offline (tpEmis 9) quando
// "voltou". queryAuthorized decide o que a consulta diz da online que ficou sem resposta.
func offlineSefaz(back *atomic.Bool, queryAuthorized bool) *fakeSefaz {
	f := &fakeSefaz{}
	f.authorize = func(key string) (sefaz.AuthorizeResult, error) {
		if key[34] == '1' || !back.Load() {
			return sefaz.AuthorizeResult{}, sefaz.ErrUncertain
		}
		return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 100, "Autorizado o uso da NF-e")}, nil
	}
	f.query = func(key string) (sefaz.QueryResult, error) {
		if !back.Load() {
			return sefaz.QueryResult{}, sefaz.ErrUncertain
		}
		if queryAuthorized {
			return sefaz.QueryResult{CStat: 100, Protocol: protocol(key, 100, "Autorizado o uso da NF-e")}, nil
		}
		return sefaz.QueryResult{CStat: 217, Motive: "NF-e não consta na base de dados da SEFAZ"}, nil
	}
	return f
}

func TestNFCeOfflineQuandoASefazNaoResponde(t *testing.T) {
	db := openIntegrationDB(t)
	var back atomic.Bool
	f := offlineSefaz(&back, true)
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)

	// A SEFAZ não respondeu: o caixa recebe na hora uma NFC-e offline.
	rec, inv := issue(t, h, "caixa-9-0001", nfceBody(issuerID, "01", "20.00"))
	if rec.Code != http.StatusCreated || inv["status"] != "offline" {
		t.Fatalf("venda sem SEFAZ: %d %v", rec.Code, inv)
	}
	key := inv["access_key"].(string)
	if key[34] != '9' || inv["number"].(float64) != 2 { // a online ficou com o 1
		t.Errorf("offline: chave %s, número %v", key, inv["number"])
	}
	var signed []byte
	_ = db.QueryRow(`SELECT signed_xml FROM invoice_invoices WHERE access_key = $1`, key).Scan(&signed)
	if s := string(signed); !strings.Contains(s, "<tpEmis>9</tpEmis>") || !strings.Contains(s, "|3|2|") || !strings.Contains(s, "<dhCont>") {
		t.Error("XML offline sem tpEmis 9, dhCont ou QR Code offline")
	}
	nfetest.MustValidateXSD(t, signed)

	// O caixa repete a chamada: recebe a mesma offline, sem nada novo.
	if _, again := issue(t, h, "caixa-9-0001", nfceBody(issuerID, "01", "20.00")); again["access_key"] != key {
		t.Errorf("repetição: %v", again["access_key"])
	}

	// A SEFAZ voltou: o worker transmite a offline e descobre que a online tinha sido
	// autorizada — cancela por substituição.
	back.Store(true)
	srv, _ := server.NewServer(db, server.WithSefaz(f))
	res := srv.Invoices().TransmitOffline(context.Background(), 50)
	if res.Authorized != 1 || res.Substituted != 1 || len(res.Errors) != 0 {
		t.Fatalf("transmissão: %+v", res)
	}
	var offStatus, onStatus string
	_ = db.QueryRow(`SELECT status FROM invoice_invoices WHERE access_key = $1`, key).Scan(&offStatus)
	_ = db.QueryRow(`SELECT status FROM invoice_invoices WHERE superseded_by IS NOT NULL`).Scan(&onStatus)
	var subst int
	_ = db.QueryRow(`SELECT COUNT(*) FROM invoice_events WHERE type = '110112' AND status = 'registered'`).Scan(&subst)
	if offStatus != "authorized" || onStatus != "canceled" || subst != 1 {
		t.Errorf("offline=%s online=%s substituição=%d", offStatus, onStatus, subst)
	}
}

// A online sem resposta nunca chegou à SEFAZ (217): o número dela é liberado e a varredura
// o inutiliza. A venda fica só com a offline.
func TestNFCeOfflineOnlineNuncaChegou(t *testing.T) {
	db := openIntegrationDB(t)
	var back atomic.Bool
	f := offlineSefaz(&back, false)
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)
	if _, inv := issue(t, h, "caixa-9-0002", nfceBody(issuerID, "01", "20.00")); inv["status"] != "offline" {
		t.Fatalf("venda: %v", inv)
	}
	// Enquanto a SEFAZ está fora, a varredura não toca em nenhum dos dois números.
	if _, sweep := do(t, h, http.MethodPost, "/v1/admin/void-sweep?older_than_hours=0", adminToken, nil); sweep["voided"].(float64) != 0 {
		t.Errorf("varredura com a SEFAZ fora: %v", sweep)
	}
	back.Store(true)
	srv, _ := server.NewServer(db, server.WithSefaz(f))
	if res := srv.Invoices().TransmitOffline(context.Background(), 50); res.Authorized != 1 || res.Released != 1 {
		t.Fatalf("transmissão: %+v", res)
	}
	_, sweep := do(t, h, http.MethodPost, "/v1/admin/void-sweep?older_than_hours=0", adminToken, nil)
	if sweep["voided"].(float64) != 1 {
		t.Errorf("o número da online deveria ser inutilizado: %v", sweep)
	}
}
