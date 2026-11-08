# Instruções para agentes

## Antes de escrever qualquer linha

Leia **[CONSTITUTION.md](./CONSTITUTION.md)**. Ele é lei, não sugestão. Um PR que viola a
constituição é rejeitado mesmo que funcione.

Os artigos que mais são violados por descuido:

1. **Art. 1**: nenhum módulo importa outro módulo. A comunicação é por porta declarada no
   próprio módulo, com um adapter em `shared/server/`.
2. **Art. 2**: nenhum `JOIN` cruza a fronteira de módulo. Use snapshot, evento ou porta.
3. **Art. 3.2**: o `pkg/` nunca importa `internal/` nem `shared/`.
4. **Art. 5**: `float64` é proibido em número fiscal. Use `pkg/decimal`.
5. **Art. 6.1**: nenhuma nota é reenviada sem antes consultar pela chave.
6. **Art. 12**: alíquota é dado, nunca código.

`go test ./internal/` verifica os artigos 1, 2, 3.1, 3.2 e 4.5 e falha o build.

## Ao criar um módulo novo

Copie a estrutura de `internal/modules/issuer/`, que é a referência canônica. A ordem é:

1. `domain/`: value objects, entidade com comportamento, factory e `error.go`.
2. `application/`: ports (output primeiro), commands, queries, DTOs, use cases (um por
   arquivo) e o service.
3. `infrastructure/`: repositório em SQL puro, model sem tags, migration numerada.
4. `adapter/http/`: request, response, mapper, handler fino e routes.
5. `wire.go`.
6. Registre a migration em `cmd/api/migrate.go`, a tabela em `tableOwners`
   (`internal/sql_boundary_test.go`) e o módulo em `shared/server/server.go`.
7. Documente a tabela em [docs/02-mapa-do-banco.md](docs/02-mapa-do-banco.md).

## Documentação

| Arquivo | O que responde |
|---|---|
| [CONSTITUTION.md](./CONSTITUTION.md) | As regras invioláveis |
| [docs/00-plano.md](docs/00-plano.md) | O que vem em cada fase, os módulos e a API |
| [docs/01-referencias.md](docs/01-referencias.md) | Os projetos e as normas usados como base, e o que se pode reaproveitar de cada um |
| [docs/02-mapa-do-banco.md](docs/02-mapa-do-banco.md) | Cada tabela, de que módulo é e o que guarda |

## Comandos

```bash
make setup   # .env + dependências
make dev     # banco + migrations + API
make check   # gofmt, vet e a suíte inteira com Postgres 18 descartável (precisa do Docker)
make test    # só os testes sem banco — rápido
make publish # check + imagem multi-arch no Docker Hub
```

Testes de integração rodam **só** pelo `make check`, num Postgres descartável. Nunca
aponte `INTEGRATION_DSN` para o banco de desenvolvimento.

## Idioma

Comentários e mensagens de erro em **português**. Identificadores de código em **inglês**.
Os campos do leiaute (`vProd`, `cClassTrib`) mantêm o nome do manual nos comentários, para
a conferência contra o MOC ser direta.
