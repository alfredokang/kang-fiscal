package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
)

func nfceBody(issuerID, method, amount string) map[string]any {
	return map[string]any{
		"issuer_id": issuerID, "model": 65, "nature_of_operation": "VENDA AO CONSUMIDOR",
		"items": []map[string]any{{"code": "CAFE-COADO", "description": "CAFE COADO 200ML", "ncm": "21011110",
			"unit": "UN", "quantity": "3", "unit_price": "6.50", "tax_profile": "venda-sp"}},
		"payments": []map[string]any{{"method": method, "amount": amount}},
	}
}

// Venda de balcão: NFC-e sem destinatário, em dinheiro, com troco.
func TestNFCeVendaDeBalcao(t *testing.T) {
	db := openIntegrationDB(t)
	h := newServerWithSefaz(t, db, &fakeSefaz{})
	issuerID := setupEmitter(t, h)

	rec, inv := issue(t, h, "caixa-1-0001", nfceBody(issuerID, "01", "20.00"))
	if rec.Code != http.StatusCreated || inv["status"] != "authorized" || inv["model"].(float64) != 65 {
		t.Fatalf("NFC-e: %d %v", rec.Code, inv)
	}
	key := inv["access_key"].(string)
	if key[20:22] != "65" || inv["number"].(float64) != 1 { // numeração própria do modelo 65
		t.Errorf("chave %s, número %v", key, inv["number"])
	}
	var signed []byte
	_ = db.QueryRow(`SELECT signed_xml FROM invoice_invoices WHERE access_key = $1`, key).Scan(&signed)
	s := string(signed)
	for _, want := range []string{
		"<infNFeSupl><qrCode>https://www.homologacao.nfce.fazenda.sp.gov.br/qrcode?p=" + key + "|3|2</qrCode>",
		"<xProd>NOTA FISCAL EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL</xProd>",
		"<tpImp>4</tpImp>", "<indFinal>1</indFinal>", "<vTroco>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("o XML não traz %q", want)
		}
	}
	if strings.Contains(s, "<dest>") {
		t.Error("consumidor não identificado não leva destinatário")
	}
	nfetest.MustValidateXSD(t, signed)

	// Cartão leva o grupo card.
	_, card := issue(t, h, "caixa-1-0002", nfceBody(issuerID, "03", "19.50"))
	var cardXML []byte
	_ = db.QueryRow(`SELECT signed_xml FROM invoice_invoices WHERE access_key = $1`, card["access_key"]).Scan(&cardXML)
	if !strings.Contains(string(cardXML), "<card><tpIntegra>2</tpIntegra></card>") {
		t.Error("pagamento no cartão sem o grupo card")
	}
	nfetest.MustValidateXSD(t, cardXML)
}

func TestNFCeInterestadualRecusada(t *testing.T) {
	db := openIntegrationDB(t)
	h := newServerWithSefaz(t, db, &fakeSefaz{})
	issuerID := setupEmitter(t, h)
	body := nfceBody(issuerID, "01", "20.00")
	body["recipient"] = map[string]any{"cpf": "52998224725", "name": "CONSUMIDOR",
		"address": map[string]any{"street": "RUA X", "number": "1", "district": "C", "city_code": "3106200",
			"city_name": "BELO HORIZONTE", "uf": "MG", "zip_code": "30110000"}}
	if rec, out := issue(t, h, "caixa-1-0003", body); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("NFC-e para outra UF: %d %v", rec.Code, out)
	}
}
