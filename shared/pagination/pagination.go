// Package pagination concentra o contrato único de paginação do serviço.
//
// A Constituição (Art. 7) proíbe qualquer consulta de lista sem LIMIT — API pública,
// admin, porta entre módulos, worker ou relatório. Este pacote existe para que essa
// regra seja o caminho mais fácil: quem usa Parse não consegue produzir uma query
// sem limite, e quem ordena por um campo é obrigado a declarar a allowlist.
package pagination

import (
	"net/http"
	"strconv"
	"strings"
)

const (
	// DefaultLimit é o tamanho de página quando o cliente não pede nada.
	DefaultLimit = 20
	// MaxLimit é o teto absoluto. Pedidos acima disso são clampados, não recusados —
	// devolver 400 para ?limit=500 quebra integração de terceiro por um detalhe que o
	// servidor sabe resolver sozinho.
	MaxLimit = 100
)

// Params são os parâmetros de paginação já validados.
type Params struct {
	Page  int
	Limit int
	Sort  string
	Order string // "asc" | "desc"
}

// Offset converte página em offset para o SQL.
func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}

// OrderBy devolve o trecho de ordenação pronto para interpolar.
//
// Só é seguro porque Sort já passou pela allowlist em Parse e Order é um de dois
// literais. Interpolar campo de ordenação vindo do usuário sem allowlist é injeção de
// SQL pela porta dos fundos — é o motivo de esta função não aceitar string livre.
func (p Params) OrderBy() string {
	return p.Sort + " " + strings.ToUpper(p.Order)
}

// Parse lê ?page, ?limit, ?sort e ?order da requisição.
//
// allowedSorts é a allowlist de colunas ordenáveis do endpoint; o primeiro item é o
// padrão. Valor fora da lista cai no padrão em vez de virar erro: o cliente que digitou
// ?sort=nome numa lista que ordena por created_at recebe uma lista, não um 400.
func Parse(r *http.Request, allowedSorts ...string) Params {
	q := r.URL.Query()

	page, err := strconv.Atoi(q.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	sort := ""
	if len(allowedSorts) > 0 {
		sort = allowedSorts[0]
		requested := q.Get("sort")
		for _, allowed := range allowedSorts {
			if requested == allowed {
				sort = requested
				break
			}
		}
	}

	order := "desc"
	if strings.EqualFold(q.Get("order"), "asc") {
		order = "asc"
	}

	return Params{Page: page, Limit: limit, Sort: sort, Order: order}
}

// Meta é o bloco de paginação devolvido em toda resposta de lista.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// NewMeta monta o bloco a partir dos parâmetros e do total contado.
func NewMeta(p Params, total int64) Meta {
	totalPages := 0
	if p.Limit > 0 {
		totalPages = int((total + int64(p.Limit) - 1) / int64(p.Limit))
	}
	return Meta{
		Page:       p.Page,
		Limit:      p.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    p.Page < totalPages,
		HasPrev:    p.Page > 1,
	}
}

// Page é o envelope padrão de toda lista da API.
type Page[T any] struct {
	Data       []T  `json:"data"`
	Pagination Meta `json:"pagination"`
}

// NewPage monta o envelope. Data nunca é nil — lista vazia serializa como [], não como
// null, porque null quebra o .map() de quem consome.
func NewPage[T any](items []T, p Params, total int64) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Data: items, Pagination: NewMeta(p, total)}
}

// OrderOrDefault devolve a ordenação pedida ou, vazia, a primeira coluna do SELECT.
//
// Os repositórios interpolam a ordenação em `ORDER BY %s`. Quem chama pela HTTP sempre
// passa uma (Parse tem padrão), mas quem chama de DENTRO — a conferência de margem que
// percorre os cupons ativos, por exemplo — às vezes não, e a consulta virava
// `ORDER BY LIMIT`: erro de sintaxe, e o cadastro que dependia dela caía com 500.
//
// `1` é a primeira coluna selecionada, que nos repositórios daqui é o id: serve em
// qualquer tabela sem que esta função precise saber quais colunas ela tem.
func OrderOrDefault(orderBy string) string {
	if strings.TrimSpace(orderBy) == "" {
		return "1"
	}
	return orderBy
}
