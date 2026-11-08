package server_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate"
	certcmd "github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/command"
	certout "github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/port/output"
	pkgcert "github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/secretbox"
)

const testKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // 32 bytes, só para teste

func TestCicloDoCertificado(t *testing.T) {
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	config.Env.SecretsEncryptionKey = testKey
	h := newHandler(t, db)

	rec, issuer := do(t, h, http.MethodPost, "/v1/admin/issuers", adminToken, map[string]any{
		"cnpj": "11222333000181", "legal_name": "Café Exemplo LTDA", "tax_regime": 3,
		"address": map[string]any{"uf": "SP"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("emitente: %d %s", rec.Code, rec.Body)
	}
	id := issuer["id"].(string)
	path := "/v1/admin/issuers/" + id + "/certificate"

	upload := func(cnpj, password string) (int, map[string]any) {
		pfx, err := pkgcert.TestPFX("CAFE EXEMPLO LTDA", cnpj, "senha-do-a1", time.Now().Add(400*24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		rec, out := do(t, h, http.MethodPut, path, adminToken, map[string]any{
			"pfx_base64": base64.StdEncoding.EncodeToString(pfx), "password": password,
		})
		return rec.Code, out
	}

	if code, _ := upload("99888777000166", "senha-do-a1"); code != http.StatusUnprocessableEntity {
		t.Errorf("certificado de outra empresa: %d", code)
	}
	if code, _ := upload("11222333000181", "errada"); code != http.StatusUnprocessableEntity {
		t.Errorf("senha errada: %d", code)
	}
	code, first := upload("11222333000181", "senha-do-a1")
	if code != http.StatusOK || first["cnpj"] != "11222333000181" || first["expiring_soon"] != false {
		t.Fatalf("upload: %d %v", code, first)
	}
	// A resposta nunca devolve o segredo.
	for _, k := range []string{"pfx_base64", "password", "sealed_pfx"} {
		if _, ok := first[k]; ok {
			t.Errorf("a resposta trouxe %s", k)
		}
	}

	// No banco, o segredo está cifrado — e a senha não aparece em lugar nenhum.
	var sealedPFX, sealedPassword string
	if err := db.QueryRow(`SELECT sealed_pfx, sealed_password FROM certificate_certificates WHERE active`).
		Scan(&sealedPFX, &sealedPassword); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sealedPFX, "v1:") || !strings.HasPrefix(sealedPassword, "v1:") ||
		strings.Contains(sealedPassword, "senha-do-a1") {
		t.Errorf("segredo em claro no banco: %q", sealedPassword)
	}

	// Trocar o certificado apaga o segredo do antigo.
	if code, _ := upload("11222333000262", "senha-do-a1"); code != http.StatusOK {
		t.Fatalf("troca: %d", code)
	}
	var active, retiredWithSecret int
	_ = db.QueryRow(`SELECT COUNT(*) FILTER (WHERE active), COUNT(*) FILTER (WHERE NOT active AND sealed_pfx IS NOT NULL)
		FROM certificate_certificates`).Scan(&active, &retiredWithSecret)
	if active != 1 || retiredWithSecret != 0 {
		t.Errorf("ativos=%d aposentados com segredo=%d", active, retiredWithSecret)
	}

	if rec, out := do(t, h, http.MethodGet, path, adminToken, nil); rec.Code != http.StatusOK || out["cnpj"] != "11222333000262" {
		t.Errorf("leitura: %d %v", rec.Code, out)
	}

	// O assinante que sai do banco assina de verdade.
	box, _ := secretbox.New(testKey)
	mod := certificate.NewModule(db, box, stubIssuers{})
	signer, err := mod.CertificateService.Signer(context.Background(), certcmd.IssuerCertificate{TenantID: "default", IssuerRef: id})
	if err != nil {
		t.Fatalf("assinante: %v", err)
	}
	signed, err := xmldsig.Sign([]byte(`<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe1" versao="4.00"></infNFe></NFe>`), "infNFe", signer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xmldsig.Verify(signed); err != nil {
		t.Errorf("assinatura com o certificado guardado: %v", err)
	}

	if rec, _ := do(t, h, http.MethodDelete, path, adminToken, nil); rec.Code != http.StatusNoContent {
		t.Errorf("revogação: %d", rec.Code)
	}
	if rec, _ := do(t, h, http.MethodGet, path, adminToken, nil); rec.Code != http.StatusNotFound {
		t.Errorf("depois de revogar: %d", rec.Code)
	}
}

// Sem SECRETS_ENCRYPTION_KEY o certificado é recusado: guardar em claro não é alternativa.
func TestCertificadoSemChaveDeCifra(t *testing.T) {
	db := openIntegrationDB(t)
	config.Env.AdminToken = adminToken
	config.Env.SecretsEncryptionKey = ""
	h := newHandler(t, db)
	_, issuer := do(t, h, http.MethodPost, "/v1/admin/issuers", adminToken, map[string]any{
		"cnpj": "11222333000181", "legal_name": "X", "tax_regime": 1, "address": map[string]any{"uf": "SP"},
	})
	pfx, _ := pkgcert.TestPFX("X", "11222333000181", "s", time.Now().Add(time.Hour))
	rec, _ := do(t, h, http.MethodPut, "/v1/admin/issuers/"+issuer["id"].(string)+"/certificate", adminToken,
		map[string]any{"pfx_base64": base64.StdEncoding.EncodeToString(pfx), "password": "s"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("sem chave: %d", rec.Code)
	}
}

type stubIssuers struct{}

func (stubIssuers) IssuerForCertificate(context.Context, string, string) (certout.IssuerSnapshot, error) {
	return certout.IssuerSnapshot{}, nil
}
