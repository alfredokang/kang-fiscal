package server_test

import (
	"net/http"
	"testing"

	"github.com/alfredokang/kang-fiscal/shared/config"
)

func TestMotorTributario(t *testing.T) {
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	h := newHandler(t, db)

	profile := map[string]any{
		"name": "Venda de café em SP",
		"rules": map[string]any{
			"cfop_internal": "5102", "cfop_interstate": "6102",
			"icms":                       map[string]any{"code": "00", "rate": "18"},
			"pis":                        map[string]any{"cst": "01", "rate": "1.65"},
			"cofins":                     map[string]any{"cst": "01", "rate": "7.6"},
			"ibs_cbs":                    map[string]any{"cst": "000", "class_trib": "000001"},
			"exclude_icms_from_pis_base": true,
		},
	}
	if rec, out := do(t, h, http.MethodPut, "/v1/admin/tax-profiles/venda-cafe-sp", adminToken, profile); rec.Code != http.StatusOK {
		t.Fatalf("perfil: %d %v", rec.Code, out)
	}
	bad := map[string]any{"name": "ST", "rules": map[string]any{"cfop_internal": "5405", "cfop_interstate": "6405",
		"icms": map[string]any{"code": "10"}, "pis": map[string]any{"cst": "01"}, "cofins": map[string]any{"cst": "01"}}}
	if rec, _ := do(t, h, http.MethodPut, "/v1/admin/tax-profiles/st", adminToken, bad); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("CST que o motor não calcula: %d", rec.Code)
	}

	simulate := func(date string) (int, map[string]any) {
		rec, out := do(t, h, http.MethodPost, "/v1/admin/tax/simulate", adminToken, map[string]any{
			"date": date, "uf": "SP", "city_code": "3550308",
			"items": []map[string]any{{"profile": "venda-cafe-sp", "quantity": "3", "unit_price": "33.33"}},
		})
		return rec.Code, out
	}
	code, sim := simulate("2026-11-08")
	if code != http.StatusOK {
		t.Fatalf("simulação: %d %v", code, sim)
	}
	totals := sim["totals"].(map[string]any)
	// Os mesmos números do pkg/tax: ICMS 18,00, PIS 1,35, COFINS 6,23, CBS 0,67, IBS 0,07.
	want := map[string]string{"v_prod": "99.99", "v_icms": "18.00", "v_pis": "1.35", "v_cofins": "6.23",
		"v_nf": "99.99", "v_cbs": "0.67", "v_ibs": "0.07", "v_nf_tot": "100.73"}
	for k, v := range want {
		if totals[k] != v {
			t.Errorf("%s = %v, esperava %s", k, totals[k], v)
		}
	}

	// 2027 não tem alíquota cadastrada: a resposta diz isso, não inventa.
	if code, out := simulate("2027-03-01"); code != http.StatusUnprocessableEntity || out["code"] != "TAX_RATE_MISSING" {
		t.Errorf("sem alíquota em 2027: %d %v", code, out)
	}

	// Sobrepor 2026 é recusado pelo banco.
	rec, _ := do(t, h, http.MethodPost, "/v1/admin/tax-rates", adminToken, map[string]any{
		"kind": "cbs", "value": "8.8", "valid_from": "2026-12-01", "source": "teste"})
	if rec.Code != http.StatusConflict {
		t.Errorf("sobreposição: %d", rec.Code)
	}

	// Cadastradas as de 2027 (valores de exemplo do teste, não oficiais), a simulação passa.
	for _, r := range []map[string]any{
		{"kind": "cbs", "value": "8.8", "valid_from": "2027-01-01", "source": "teste"},
		{"kind": "ibs_uf", "value": "0.05", "valid_from": "2027-01-01", "source": "teste"},
		{"kind": "ibs_mun", "value": "0.05", "valid_from": "2027-01-01", "source": "teste"},
	} {
		if rec, out := do(t, h, http.MethodPost, "/v1/admin/tax-rates", adminToken, r); rec.Code != http.StatusCreated {
			t.Fatalf("alíquota 2027: %d %v", rec.Code, out)
		}
	}
	if code, out := simulate("2027-03-01"); code != http.StatusOK || out["rates"].(map[string]any)["cbs"] != "8.8000" {
		t.Errorf("simulação 2027: %d %v", code, out)
	}
}
