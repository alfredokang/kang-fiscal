package server_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
)

// A mesma venda chegando dez vezes AO MESMO TEMPO (o cliente que não esperou a resposta)
// vira uma nota só, sem gastar número: antes, cada requisição que perdia a corrida deixava
// um buraco na numeração e respondia 409 com a mensagem errada.
func TestMesmaVendaSimultaneaNaoGastaNumero(t *testing.T) {
	db := openIntegrationDB(t)
	f := &fakeSefaz{}
	f.authorize = func(key string) (sefaz.AuthorizeResult, error) {
		time.Sleep(50 * time.Millisecond) // a SEFAZ demora: as requisições se sobrepõem
		return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 100, "Autorizado o uso da NF-e")}, nil
	}
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)

	var wg sync.WaitGroup
	codes := make(chan int, 10)
	ids := make(chan string, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec, inv := issue(t, h, "venda-repetida", issueBody(issuerID, "09012100"))
			codes <- rec.Code
			id, _ := inv["id"].(string)
			ids <- id
		}()
	}
	wg.Wait()
	close(codes)
	close(ids)
	for c := range codes {
		if c != http.StatusCreated && c != http.StatusAccepted {
			t.Errorf("resposta %d para a mesma venda", c)
		}
	}
	seen := map[string]bool{}
	for id := range ids {
		seen[id] = true
	}
	if len(seen) != 1 {
		t.Errorf("a mesma venda devolveu %d notas diferentes", len(seen))
	}
	var notes, reservations int
	_ = db.QueryRow(`SELECT COUNT(*) FROM invoice_invoices`).Scan(&notes)
	_ = db.QueryRow(`SELECT COUNT(*) FROM numbering_reservations`).Scan(&reservations)
	if notes != 1 || reservations != 1 || f.authorizeCalls.Load() != 1 {
		t.Errorf("notas=%d reservas=%d envios=%d; esperava 1, 1 e 1", notes, reservations, f.authorizeCalls.Load())
	}
	// A próxima venda pega o número 2: nenhum número ficou para trás.
	if _, inv := issue(t, h, "venda-seguinte", issueBody(issuerID, "09012100")); inv["number"].(float64) != 2 {
		t.Errorf("próxima venda: número %v", inv["number"])
	}
}
