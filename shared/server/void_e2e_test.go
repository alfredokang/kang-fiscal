package server_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/numbering"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
)

func TestVarreduraInutilizaSoOsBuracos(t *testing.T) {
	db := openIntegrationDB(t)
	var mode atomic.Int32 // 0 rejeita, 1 autoriza, 2 sem resposta
	f := &fakeSefaz{}
	f.authorize = func(key string) (sefaz.AuthorizeResult, error) {
		switch mode.Load() {
		case 0:
			return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 778, "Rejeição: Informado NCM inexistente")}, nil
		case 1:
			return sefaz.AuthorizeResult{BatchCStat: 104, Protocol: protocol(key, 100, "Autorizado o uso da NF-e")}, nil
		}
		return sefaz.AuthorizeResult{}, sefaz.ErrUncertain
	}
	f.query = func(string) (sefaz.QueryResult, error) { return sefaz.QueryResult{}, sefaz.ErrUncertain }
	h := newServerWithSefaz(t, db, f)
	issuerID := setupEmitter(t, h)

	mode.Store(0)
	if _, inv := issue(t, h, "v-1", issueBody(issuerID, "09019999")); inv["status"] != "rejected" {
		t.Fatalf("nº 1: %v", inv)
	}
	mode.Store(1)
	if _, inv := issue(t, h, "v-2", issueBody(issuerID, "09012100")); inv["status"] != "authorized" {
		t.Fatalf("nº 2: %v", inv)
	}
	// nº 3: reservado e sem nota — a queda entre a reserva e a gravação.
	res, err := numbering.NewModule(db, allIssuers{}).Service.Reserve(context.Background(), entity.SeriesKey{
		TenantID: "default", IssuerRef: issuerID, Model: 55, Series: 1, Environment: "homologation"}, uuid.NewString())
	if err != nil || res.Number != 3 {
		t.Fatalf("nº 3: %v %v", res, err)
	}
	mode.Store(2)
	if _, inv := issue(t, h, "v-4", issueBody(issuerID, "09012100")); inv["status"] != "uncertain" {
		t.Fatalf("nº 4: %v", inv)
	}

	// A faixa manual com a nota autorizada no meio é recusada.
	if rec, _ := do(t, h, http.MethodPost, "/v1/nfe/void-range", adminToken, map[string]any{"issuer_id": issuerID,
		"series": 1, "from": 1, "to": 3, "justification": "Numeracao pulada por falha no sistema"}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("faixa com nota autorizada: %d", rec.Code)
	}

	rec, sweep := do(t, h, http.MethodPost, "/v1/admin/void-sweep?older_than_hours=0", adminToken, nil)
	if rec.Code != http.StatusOK || sweep["voided"].(float64) != 2 || sweep["skipped"].(float64) != 1 {
		t.Fatalf("varredura: %d %v", rec.Code, sweep)
	}
	// 1 e 3 não são seguidos (o 2 é da nota autorizada): duas faixas, dois pedidos.
	if f.voidCalls.Load() != 2 {
		t.Errorf("pedidos de inutilização: %d", f.voidCalls.Load())
	}
	var voided, open int
	_ = db.QueryRow(`SELECT COUNT(*) FILTER (WHERE status = 'voided'), COUNT(*) FILTER (WHERE status = 'reserved')
		FROM numbering_reservations`).Scan(&voided, &open)
	if voided != 2 || open != 1 { // o nº 4, incerto, continua aberto
		t.Errorf("reservas: inutilizadas=%d abertas=%d", voided, open)
	}
	_, list := do(t, h, http.MethodGet, "/v1/nfe/voids", adminToken, nil)
	if n := len(list["data"].([]any)); n != 2 {
		t.Errorf("inutilizações: %d", n)
	}
	// A inutilização homologada não muda.
	if _, err := db.Exec(`UPDATE invoice_voids SET justification = 'x'`); err == nil {
		t.Error("inutilização homologada alterada")
	}

	// Uma segunda varredura não pede nada de novo.
	_, again := do(t, h, http.MethodPost, "/v1/admin/void-sweep?older_than_hours=0", adminToken, nil)
	if again["voided"].(float64) != 0 || f.voidCalls.Load() != 2 {
		t.Errorf("segunda varredura: %v", again)
	}
}
