# Contribuindo com o Kang Fiscal

Obrigado por querer ajudar. Nota fiscal mexe com o dinheiro e com a obrigação legal de
quem emite, então o padrão aqui é alto — mas o caminho é simples.

## Antes de começar

1. Leia a [Constituição](CONSTITUTION.md). Um pull request que a viola é recusado mesmo que
   funcione. As regras que mais pegam: nenhum módulo importa outro, nenhum `JOIN` entre
   módulos, nenhum `float64` em valor fiscal, nenhuma lista sem `LIMIT`.
2. Para mudança grande (módulo novo, mudança de comportamento na emissão), abra uma issue
   antes e descreva o que pretende. Para correção pequena, pode mandar o PR direto.

## Ambiente

Go 1.26, Docker e `xmllint` (pacote `libxml2-utils` no Debian/Ubuntu; já vem no macOS).

```bash
make setup   # .env e dependências
make dev     # Postgres + migrations + API
make check   # o que o PR precisa passar: gofmt, vet e a suíte inteira
```

O `make check` sobe um Postgres 18 descartável; os testes nunca tocam no seu banco de
desenvolvimento.

## O que um PR precisa ter

- **Testes.** Mudança na emissão vem com teste de ponta a ponta (veja `shared/server/*_e2e_test.go`,
  que simulam a SEFAZ). XML novo é validado contra o XSD oficial.
- **`make check` verde.**
- **Rota nova documentada** em `shared/openapi/openapi.yaml` — o build quebra se faltar.
- **Tabela nova registrada** em `internal/sql_boundary_test.go` e em
  [docs/02-mapa-do-banco.md](docs/02-mapa-do-banco.md).
- **Idioma:** comentários e mensagens de erro em português; identificadores em inglês.
  Comentário explica o *porquê*, não o *quê*.

## Código de terceiros

Pode reaproveitar código MIT, BSD ou Apache-2.0, mantendo o aviso de copyright original
no arquivo e acrescentando o crédito no [NOTICE](NOTICE). Código LGPL, GPL ou sem licença
**não** entra: estude o desenho e escreva do zero.

## Licença

Ao contribuir, você concorda que sua contribuição é distribuída sob a
[Licença Apache 2.0](LICENSE), como o resto do projeto.
