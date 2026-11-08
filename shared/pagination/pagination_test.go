package pagination_test

import (
	"net/http/httptest"
	"testing"

	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

func TestParse(t *testing.T) {
	casos := []struct {
		nome      string
		url       string
		wantLimit int
		wantPage  int
		wantSort  string
		wantOrder string
	}{
		{"padrões", "/x", pagination.DefaultLimit, 1, "created_at", "desc"},
		{"clampa acima do máximo", "/x?limit=5000", pagination.MaxLimit, 1, "created_at", "desc"},
		{"ignora página inválida", "/x?page=-3", pagination.DefaultLimit, 1, "created_at", "desc"},
		{"aceita sort da allowlist", "/x?sort=name&order=asc", pagination.DefaultLimit, 1, "name", "asc"},
		{"recusa sort fora da allowlist", "/x?sort=senha", pagination.DefaultLimit, 1, "created_at", "desc"},
		{"lixo em limit cai no padrão", "/x?limit=abc", pagination.DefaultLimit, 1, "created_at", "desc"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := pagination.Parse(httptest.NewRequest("GET", c.url, nil), "created_at", "name", "email")
			if got.Limit != c.wantLimit || got.Page != c.wantPage || got.Sort != c.wantSort || got.Order != c.wantOrder {
				t.Errorf("Parse(%s) = %+v", c.url, got)
			}
		})
	}
}

func TestNewMeta(t *testing.T) {
	meta := pagination.NewMeta(pagination.Params{Page: 2, Limit: 20}, 137)
	if meta.TotalPages != 7 {
		t.Errorf("esperava 7 páginas para 137 itens a 20 por página, veio %d", meta.TotalPages)
	}
	if !meta.HasNext || !meta.HasPrev {
		t.Error("página 2 de 7 deveria ter anterior e próxima")
	}
}

func TestNewPageNuncaDevolveNil(t *testing.T) {
	// Lista vazia tem que serializar como [], não como null: null quebra o .map() do front.
	page := pagination.NewPage[string](nil, pagination.Params{Page: 1, Limit: 20}, 0)
	if page.Data == nil {
		t.Error("Data não pode ser nil")
	}
}
