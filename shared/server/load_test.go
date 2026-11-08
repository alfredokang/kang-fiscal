package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

// TestCarga simula vendas em volume contra a API de verdade (HTTP real, Postgres 18 com o
// pool da produção) e uma SEFAZ simulada. Mede vazão e latência e, no fim, confere no
// banco e na "SEFAZ" o que importa: nenhuma venda virou duas notas e a numeração não tem
// buraco nem repetição.
//
// Roda só com `make load` (fica fora do make check: leva minutos e mede tempo).
func TestCarga(t *testing.T) {
	if os.Getenv("LOAD_TEST") != "1" {
		t.Skip("simulação de carga: rode com make load")
	}
	cases := []loadCase{
		{name: "teto do sistema (SEFAZ instantânea)", sales: 3000, clients: 100, series: 4},
		{name: "SEFAZ realista (200 a 800 ms)", sales: 3000, clients: 200, series: 4, minLatency: 200 * time.Millisecond, maxLatency: 800 * time.Millisecond},
		{name: "caos: 10% das respostas se perdem, 5% das vendas reenviadas", sales: 2000, clients: 200, series: 4,
			minLatency: 200 * time.Millisecond, maxLatency: 800 * time.Millisecond, lostRate: 0.10, retryRate: 0.05},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runLoad(t, c) })
	}
}

type loadCase struct {
	name                   string
	sales, clients, series int
	minLatency, maxLatency time.Duration
	lostRate, retryRate    float64 // resposta perdida depois de autorizar; venda reenviada pelo cliente
}

// sefazSim é a SEFAZ do outro lado: guarda o que autorizou, para provar depois que nenhuma
// venda foi autorizada duas vezes.
type sefazSim struct {
	fakeSefaz
	c          loadCase
	mu         sync.Mutex
	authorized map[string]bool // chave → autorizada
	lost       atomic.Int64
}

func (s *sefazSim) wait() {
	if s.c.maxLatency > 0 {
		time.Sleep(s.c.minLatency + time.Duration(rand.Int64N(int64(s.c.maxLatency-s.c.minLatency))))
	}
}

func (s *sefazSim) Authorize(_ context.Context, _ *certificate.Certificate, _ sefaz.Target, _ int64, signed []byte) (sefaz.AuthorizeResult, error) {
	s.wait()
	key := keyOf(signed)
	s.mu.Lock()
	dup := s.authorized[key]
	s.authorized[key] = true
	s.mu.Unlock()
	if dup {
		return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 204, "Rejeição: Duplicidade de NF-e")}, nil
	}
	// Autorizou, mas a resposta não chega: o pior caso para nota duplicada.
	if rand.Float64() < s.c.lostRate {
		s.lost.Add(1)
		return sefaz.AuthorizeResult{}, sefaz.ErrUncertain
	}
	return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 100, "Autorizado o uso da NF-e")}, nil
}

func (s *sefazSim) Query(_ context.Context, _ *certificate.Certificate, _ sefaz.Target, key string) (sefaz.QueryResult, error) {
	s.wait()
	s.mu.Lock()
	ok := s.authorized[key]
	s.mu.Unlock()
	if ok {
		p := protocol(key, 100, "Autorizado o uso da NF-e")
		return sefaz.QueryResult{CStat: 100, Protocol: p}, nil
	}
	return sefaz.QueryResult{CStat: 217, Motive: "Rejeição: NF-e não consta na base de dados da SEFAZ"}, nil
}

func runLoad(t *testing.T, c loadCase) {
	db := openIntegrationDB(t)
	// O mesmo pool da produção (shared/database/postgres): o gargalo tem de aparecer aqui.
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	sim := &sefazSim{c: c, authorized: map[string]bool{}}
	config.Env.AdminToken = adminToken
	config.Env.SecretsEncryptionKey = testKey
	config.Env.RateLimitPerMinute = 0
	srv, err := server.NewServer(db, server.WithSefaz(sim))
	if err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	issuerID := setupEmitter(t, h)
	httpSrv := httptest.NewServer(h)
	defer httpSrv.Close()
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: c.clients}, Timeout: 2 * time.Minute}

	type result struct {
		status  int
		latency time.Duration
	}
	jobs := make(chan int)
	results := make(chan result, c.sales*2)
	send := func(sale int) {
		body := issueBody(issuerID, "09012100")
		body["series"] = 1 + sale%c.series // uma série por caixa
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, httpSrv.URL+"/v1/nfe", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Idempotency-Key", fmt.Sprintf("venda-%d", sale))
		start := time.Now()
		resp, err := client.Do(req)
		r := result{latency: time.Since(start)}
		if err == nil {
			r.status = resp.StatusCode
			_ = resp.Body.Close()
		}
		results <- r
	}
	var wg sync.WaitGroup
	for range c.clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sale := range jobs {
				// O cliente que não esperou a resposta e mandou de novo, ao mesmo tempo.
				if rand.Float64() < c.retryRate {
					var dup sync.WaitGroup
					dup.Add(1)
					go func() { defer dup.Done(); send(sale) }()
					send(sale)
					dup.Wait()
					continue
				}
				send(sale)
			}
		}()
	}
	start := time.Now()
	for sale := range c.sales {
		jobs <- sale
	}
	close(jobs)
	wg.Wait()
	elapsed := time.Since(start)
	close(results)

	var lat []time.Duration
	codes := map[int]int{}
	for r := range results {
		lat = append(lat, r.latency)
		codes[r.status]++
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	pct := func(p float64) time.Duration { return lat[int(float64(len(lat)-1)*p)].Round(time.Millisecond) }

	// As notas incertas são resolvidas pelo worker; aqui, uma rodada da conciliação.
	if c.lostRate > 0 {
		sweep := srv.Invoices().SweepPending(context.Background(), 0, 10_000)
		t.Logf("worker: %d conferidas, %d inutilizadas, erros %v", sweep.Checked, sweep.Voided, sweep.Errors)
	}

	t.Logf("%d vendas, %d clientes simultâneos, %d séries: %s — %.0f notas/s",
		c.sales, c.clients, c.series, elapsed.Round(time.Millisecond), float64(c.sales)/elapsed.Seconds())
	t.Logf("latência por requisição: p50 %s, p95 %s, p99 %s, máx %s", pct(0.5), pct(0.95), pct(0.99), pct(1))
	t.Logf("respostas HTTP: %v; respostas perdidas na SEFAZ: %d", codes, sim.lost.Load())
	for code, n := range codes {
		if code != http.StatusCreated && code != http.StatusAccepted {
			t.Errorf("%d respostas HTTP %d: a venda repetida tem de devolver a mesma nota", n, code)
		}
	}

	// 1. Uma nota por venda, nem mais nem menos.
	var notes, authorized, keys int
	if err := db.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'authorized'), COUNT(DISTINCT access_key)
		FROM invoice_invoices`).Scan(&notes, &authorized, &keys); err != nil {
		t.Fatal(err)
	}
	t.Logf("banco: %d notas, %d autorizadas", notes, authorized)
	if notes != c.sales || keys != c.sales {
		t.Errorf("esperava %d notas (uma por venda), há %d (%d chaves distintas)", c.sales, notes, keys)
	}
	// 2. Toda nota que a SEFAZ autorizou está registrada como autorizada aqui.
	sim.mu.Lock()
	atSefaz := len(sim.authorized)
	sim.mu.Unlock()
	if atSefaz != c.sales {
		t.Errorf("a SEFAZ autorizou %d notas para %d vendas: venda duplicada ou perdida", atSefaz, c.sales)
	}
	if authorized != c.sales {
		var pending int
		_ = db.QueryRow(`SELECT COUNT(*) FROM invoice_invoices WHERE status = 'uncertain'`).Scan(&pending)
		t.Errorf("%d autorizadas de %d (%d ainda incertas)", authorized, c.sales, pending)
	}
	// 3. Numeração sem buraco nem repetição, em cada série.
	rows, err := db.Query(`SELECT series, COUNT(*), COUNT(DISTINCT number), MIN(number), MAX(number)
		FROM invoice_invoices GROUP BY series ORDER BY series`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var series, n, distinct, lo, hi int
		if err := rows.Scan(&series, &n, &distinct, &lo, &hi); err != nil {
			t.Fatal(err)
		}
		if n != distinct || lo != 1 || hi != n {
			t.Errorf("série %d: %d notas, %d números distintos, de %d a %d", series, n, distinct, lo, hi)
		}
	}
}
