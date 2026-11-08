// Package openapi embute a especificação da API e a página de documentação.
package openapi

import (
	_ "embed"
	"net/http"
)

// Spec é o openapi.yaml, servido em /openapi.yaml e base dos SDKs (sdk/).
//
//go:embed openapi.yaml
var Spec []byte

// docsPage é o Swagger UI apontando para a especificação. Os arquivos vêm do jsDelivr com
// versão fixa e hash de integridade: a página não executa script que mudou sem aviso.
const docsPage = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kang Fiscal — API</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css" integrity="sha384-wxLW6kwyHktdDGr6Pv1zgm/VGJh99lfUbzSn6HNHBENZlCN7W602k9VkGdxuFvPn" crossorigin="anonymous">
</head>
<body>
<div id="docs"></div>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js" integrity="sha384-wmyclcVGX/WhUkdkATwhaK1X1JtiNrr2EoYJ+diV3vj4v6OC5yCeSu+yW13SYJep" crossorigin="anonymous"></script>
<script>SwaggerUIBundle({url: "/openapi.yaml", dom_id: "#docs", deepLinking: true});</script>
</body>
</html>`

// Register pendura /openapi.yaml e /docs.
func Register(mux interface {
	Get(pattern string, h http.HandlerFunc)
}) {
	mux.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(Spec)
	})
	mux.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src https://cdn.jsdelivr.net 'unsafe-inline'; style-src https://cdn.jsdelivr.net; img-src data: https:; connect-src 'self'")
		_, _ = w.Write([]byte(docsPage))
	})
}
