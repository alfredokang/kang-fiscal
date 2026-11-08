package server_test

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/openapi"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

// TestOpenAPICobreTodasAsRotas é o contrapeso de escrever a especificação à mão: ela é a
// base dos SDKs, e um SDK gerado de um mapa desatualizado chama rota que não existe. As
// rotas registradas no chi e as documentadas precisam bater, nos dois sentidos.
func TestOpenAPICobreTodasAsRotas(t *testing.T) {
	registered := registeredRoutes(t)
	documented := documentedRoutes(t)
	var missing, stale []string
	for r := range registered {
		if !documented[r] {
			missing = append(missing, r)
		}
	}
	for r := range documented {
		if !registered[r] {
			stale = append(stale, r)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	for _, r := range missing {
		t.Errorf("rota registrada e NÃO documentada: %s — acrescente em shared/openapi/openapi.yaml", r)
	}
	for _, r := range stale {
		t.Errorf("rota documentada e NÃO registrada: %s — saiu do código? tire de shared/openapi/openapi.yaml", r)
	}
}

func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()
	config.Env.SecretsEncryptionKey = testKey
	// Registrar rota não toca no banco: o servidor sobe com db nulo e o teste roda sem Postgres.
	srv, err := server.NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	err = chi.Walk(srv.Handler().(chi.Router), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(route, "/")
		switch {
		case route == "/docs" || route == "/openapi.yaml":
			return nil // a documentação não se documenta
		case route == "/metrics" && method != http.MethodGet:
			return nil // Handle aceita todo verbo; só GET é contrato
		}
		out[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// documentedRoutes lê os caminhos, resolvendo os path items por $ref (as rotas de
// /v1/admin e de /v1 compartilham a mesma definição).
func documentedRoutes(t *testing.T) map[string]bool {
	t.Helper()
	var spec struct {
		Paths      map[string]map[string]any `yaml:"paths"`
		Components struct {
			PathItems map[string]map[string]any `yaml:"pathItems"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(openapi.Spec, &spec); err != nil {
		t.Fatalf("openapi.yaml inválido: %v", err)
	}
	verbs := map[string]bool{"get": true, "post": true, "put": true, "delete": true, "patch": true}
	out := map[string]bool{}
	for path, item := range spec.Paths {
		if ref, ok := item["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/components/pathItems/")
			resolved, found := spec.Components.PathItems[name]
			if !found {
				t.Errorf("%s aponta para %s, que não existe", path, ref)
				continue
			}
			item = resolved
		}
		for verb := range item {
			if verbs[verb] {
				out[strings.ToUpper(verb)+" "+path] = true
			}
		}
	}
	return out
}

// TestOpenAPIListasTemLimite guarda o Art. 7 na documentação: lista documentada sem
// `limit` convida o integrador a pedir a coleção inteira.
func TestOpenAPIListasTemLimite(t *testing.T) {
	var spec map[string]any
	if err := yaml.Unmarshal(openapi.Spec, &spec); err != nil {
		t.Fatal(err)
	}
	components := spec["components"].(map[string]any)
	resolve := func(n any) map[string]any {
		m, _ := n.(map[string]any)
		if ref, ok := m["$ref"].(string); ok {
			parts := strings.Split(strings.TrimPrefix(ref, "#/components/"), "/")
			m, _ = components[parts[0]].(map[string]any)[parts[1]].(map[string]any)
		}
		return m
	}
	for path, raw := range spec["paths"].(map[string]any) {
		get, ok := resolve(raw)["get"].(map[string]any)
		if !ok {
			continue
		}
		// Lista paginada é a que devolve `pagination`; procura no texto da resposta.
		body, _ := yaml.Marshal(resolve(get["responses"].(map[string]any)["200"]))
		if !strings.Contains(string(body), "Pagination") {
			continue
		}
		hasLimit := false
		for _, p := range get["parameters"].([]any) {
			if ref, _ := p.(map[string]any)["$ref"].(string); strings.HasSuffix(ref, "/Limit") {
				hasLimit = true
			}
		}
		if !hasLimit {
			t.Errorf("GET %s é lista paginada e não documenta `limit` (Constituição, Art. 7)", path)
		}
	}
}

// TestOpenAPINaoTemChaveRepetida: yaml.Unmarshal num mapa aceita chave repetida em
// silêncio, mas o Swagger UI recusa o arquivo inteiro. A conferência é no nó bruto.
func TestOpenAPINaoTemChaveRepetida(t *testing.T) {
	var root yaml.Node
	if err := yaml.Unmarshal(openapi.Spec, &root); err != nil {
		t.Fatal(err)
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			seen := map[string]int{}
			for i := 0; i+1 < len(n.Content); i += 2 {
				k := n.Content[i]
				if line, dup := seen[k.Value]; dup {
					t.Errorf("chave %q repetida na linha %d (a primeira na linha %d)", k.Value, k.Line, line)
				}
				seen[k.Value] = k.Line
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)
}

func TestDocsServidas(t *testing.T) {
	config.Env.SecretsEncryptionKey = testKey
	srv, err := server.NewServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"/openapi.yaml": "openapi: 3.1.0", "/docs": "swagger-ui"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}

// TestOpenAPISchemasSoComPalavrasChave pega o erro clássico do YAML inline: dentro de
// { ... }, uma vírgula numa descrição sem aspas abre um campo novo. O YAML continua válido
// (o Unmarshal aceita), mas o schema ganha uma "palavra-chave" inventada, que o Redocly e
// os geradores de SDK recusam.
func TestOpenAPISchemasSoComPalavrasChave(t *testing.T) {
	allowed := map[string]bool{}
	for _, k := range strings.Fields(`type format description enum default example examples pattern minimum maximum
		minLength maxLength minItems maxItems items properties required allOf oneOf anyOf $ref additionalProperties
		contentEncoding nullable readOnly writeOnly deprecated title const`) {
		allowed[k] = true
	}
	var spec any
	if err := yaml.Unmarshal(openapi.Spec, &spec); err != nil {
		t.Fatal(err)
	}
	var walk func(n any, path string, inProperties bool)
	walk = func(n any, path string, inProperties bool) {
		switch v := n.(type) {
		case map[string]any:
			// Um schema é um mapa com "type" (fora de "properties", onde as chaves são
			// nomes de campo, e um campo pode se chamar "type").
			if _, isSchema := v["type"]; isSchema && !inProperties {
				if _, isParam := v["in"]; !isParam {
					if _, isSec := v["scheme"]; !isSec {
						for k := range v {
							if !allowed[k] {
								t.Errorf("%s: %q não é palavra-chave de schema — vírgula sem aspas numa descrição?", path, k)
							}
						}
					}
				}
			}
			for k, c := range v {
				walk(c, path+"/"+k, k == "properties" && !inProperties)
			}
		case []any:
			for _, c := range v {
				walk(c, path, false)
			}
		}
	}
	walk(spec, "", false)
}
