package server_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/numbering"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/config"
)

type allIssuers struct{}

func (allIssuers) IssuerExists(context.Context, string, string) (bool, error) { return true, nil }

func TestNumeracaoSemBuracoNemRepeticao(t *testing.T) {
	db := openIntegrationDB(t)
	svc := numbering.NewModule(db, allIssuers{}).Service
	ctx := context.Background()
	key := entity.SeriesKey{TenantID: "default", IssuerRef: "01a1bb1a-a6ac-72fd-9d66-8078b42feefd",
		Model: 55, Series: 1, Environment: "homologation"}

	// 50 emissões ao mesmo tempo: cada uma recebe um número, todos distintos e seguidos.
	var mu sync.Mutex
	var got []int
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := svc.Reserve(ctx, key, fmt.Sprintf("doc-%d", i))
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			got = append(got, res.Number)
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Ints(got)
	for i, n := range got {
		if n != i+1 {
			t.Fatalf("números: %v", got)
		}
	}

	// Produção numera separado de homologação.
	prod := key
	prod.Environment = "production"
	if res, _ := svc.Reserve(ctx, prod, "doc-prod"); res.Number != 1 {
		t.Errorf("produção começou em %d", res.Number)
	}

	// Migração de outro sistema: continua de onde parou, e nunca volta.
	if err := svc.SetNext(ctx, key, 1532); err != nil {
		t.Fatal(err)
	}
	res, _ := svc.Reserve(ctx, key, "doc-migrado")
	if res.Number != 1532 {
		t.Errorf("depois da migração: %d", res.Number)
	}
	if err := svc.SetNext(ctx, key, 1000); !errors.Is(err, domain.ErrNumberBackward) {
		t.Errorf("voltar a numeração: %v", err)
	}

	// Número reservado e não usado aparece como buraco a inutilizar.
	if err := svc.MarkUsed(ctx, "default", res.ID); err != nil {
		t.Fatal(err)
	}
	pending, err := svc.PendingVoid(ctx, -time.Hour, 500) // corte no futuro: tudo que está aberto
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 51 { // as 50 de homologação e a de produção; a 1532 foi usada
		t.Errorf("pendentes de inutilização: %d", len(pending))
	}
	if err := svc.MarkUsed(ctx, "default", res.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("fechar duas vezes: %v", err)
	}

	if _, err := svc.Reserve(ctx, entity.SeriesKey{TenantID: "default", IssuerRef: key.IssuerRef, Model: 55,
		Series: 890, Environment: "homologation"}, "x"); !errors.Is(err, domain.ErrInvalidSeries) {
		t.Errorf("série do fisco: %v", err)
	}
}

func TestRotaDoProximoNumero(t *testing.T) {
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	h := newHandler(t, db)
	_, issuer := do(t, h, http.MethodPost, "/v1/admin/issuers", adminToken, map[string]any{
		"cnpj": "11222333000181", "legal_name": "X", "tax_regime": 1, "address": map[string]any{"uf": "SP"}})
	path := "/v1/admin/issuers/" + issuer["id"].(string) + "/series/55/1"
	if rec, out := do(t, h, http.MethodPut, path, adminToken, map[string]any{"environment": "homologation", "next_number": 200}); rec.Code != http.StatusOK {
		t.Fatalf("ajuste: %d %v", rec.Code, out)
	}
	if rec, _ := do(t, h, http.MethodPut, "/v1/admin/issuers/01a1bb1a-a6ac-72fd-9d66-000000000000/series/55/1", adminToken,
		map[string]any{"environment": "homologation", "next_number": 5}); rec.Code != http.StatusNotFound {
		t.Errorf("emitente inexistente: %d", rec.Code)
	}
}
