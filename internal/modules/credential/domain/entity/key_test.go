package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain/entity"
)

func TestCredencial(t *testing.T) {
	pepper := []byte("pepper-de-teste")
	k, token, err := entity.NewKey("id", "erp-cliente-1", "ERP", []string{"nfe:write", "nfe:read", "nfe:read"}, pepper, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	prefix, secret, ok := entity.Split(token)
	if !ok || prefix != k.Prefix || len(k.Scopes) != 2 {
		t.Fatalf("token %q, chave %+v", token, k)
	}
	if !k.Matches(pepper, secret) || k.Matches(pepper, secret[:39]+"x") || k.Matches([]byte("outro"), secret) {
		t.Error("conferência do segredo")
	}
	if k.Hash == secret {
		t.Error("o segredo foi guardado em claro")
	}
	if _, _, ok := entity.Split("kf_curto_x"); ok {
		t.Error("token malformado aceito")
	}
	for name, c := range map[string]struct {
		tenant string
		scopes []string
		pepper []byte
		want   error
	}{
		"tenant com maiúscula": {"Cliente", []string{"nfe:read"}, pepper, domain.ErrInvalidTenant},
		"escopo inventado":     {"cliente", []string{"admin"}, pepper, domain.ErrInvalidScope},
		"sem escopo":           {"cliente", nil, pepper, domain.ErrRequiredField},
		"sem pepper":           {"cliente", []string{"nfe:read"}, nil, domain.ErrNoPepper},
	} {
		if _, _, err := entity.NewKey("id", c.tenant, "X", c.scopes, c.pepper, time.Now()); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
