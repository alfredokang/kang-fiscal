# Constituição — Kang Fiscal

> Este documento é **lei**. Nenhum código entra neste repositório violando o que está aqui.
> Quando uma regra precisar mudar, muda-se **primeiro** este documento, com a justificativa,
> e só depois o código. Um PR que viola a constituição é rejeitado mesmo que funcione.
>
> Ela herda a constituição do backend da Minas Café, onde a arquitetura nasceu, e acrescenta
> o que um emissor fiscal exige: a nota não pode sair duas vezes, o XML autorizado não muda
> nunca e alíquota é dado, não código.

---

## Artigo 0 — Por que uma constituição

O Kang Fiscal é um **monolito modular**: o custo operacional de um binário só, com a
disciplina de fronteiras de microsserviços. Essa promessa só se sustenta enquanto as
fronteiras forem respeitadas **sem exceção**.

O teste de integridade é permanente:

> **Qualquer módulo deve poder ser arrancado deste repositório e virar um serviço
> independente sem reescrever sua lógica de negócio.**

E um segundo, próprio de projeto open source:

> **Quem baixa a imagem precisa só de Postgres para emitir.** Cada dependência de
> infraestrutura a mais (Redis, fila, S3) é uma barreira para o empresário pequeno que
> quer rodar isso num servidor de R$ 30. Ela só entra opcional, nunca obrigatória.

---

## Artigo 1 — Fronteiras entre módulos

### 1.1 Nenhum módulo importa outro módulo

É **proibido** qualquer import de `internal/modules/<outro>/...` dentro de
`internal/modules/<meu>/...`. `internal/architecture_test.go` falha o build.

As exceções são `shared/` (infraestrutura, nunca regra de negócio) e `pkg/` (a biblioteca
fiscal pública, que não conhece banco, HTTP nem tenant — ver Artigo 3.2).

### 1.2 A comunicação é por porta, e a porta é minha

A interface que um módulo usa para falar com outro é **declarada no módulo consumidor**,
em `application/port/output/`, com tipos **do consumidor**. Quem a implementa é um adapter
em `shared/server/*_adapter.go` — o único lugar autorizado a enxergar dois módulos.

### 1.3 Ports pequenas

Uma porta resolve **uma** necessidade. `NumberReserver` e `IssuerReader` são duas portas.

---

## Artigo 2 — Nenhum JOIN entre módulos

Uma query SQL só toca tabelas **do próprio módulo**. Proibido `JOIN`, subselect, CTE ou
view que cruze a fronteira. `internal/sql_boundary_test.go` falha o build quando um módulo
menciona tabela alheia.

Como se vive sem JOIN, nesta ordem de preferência:

1. **Snapshot na escrita.** A nota copia para dentro de si o emitente, o destinatário, os
   itens e as alíquotas do momento da emissão. Nota emitida nunca mais consulta o cadastro
   — e isso não é otimização, é correção: o documento fiscal registra o que valia naquele
   dia.
2. **Evento de domínio + read model próprio.**
3. **Porta de leitura síncrona**, sempre com limite e timeout.

Chave estrangeira entre módulos é proibida: a referência é texto opaco (`issuer_ref`).

---

## Artigo 3 — Camadas

```
internal/modules/<módulo>/
├── domain/          # regra pura: entity, valueobject, factory, event, error.go
├── application/     # command, query, dto, usecase, port/input, port/output, mapper
├── infrastructure/  # persistence/postgres/{model,mapper,migrations}, gateway
├── adapter/http/    # handler FINO, request, response, mapper, routes.go
└── wire.go
```

### 3.1 Sentido das dependências

```
adapter → application → domain
infrastructure → application → domain
```

O `domain/` não importa `database/sql`, `net/http`, `chi`, `pgx` nem `shared/`. Pode
importar `pkg/`, porque `pkg/` é tão puro quanto ele.

### 3.2 O `pkg/` é a biblioteca, os módulos são o produto

`pkg/` é a parte do Kang Fiscal que outro desenvolvedor Go importa sem subir a API:
layout da NF-e, chave de acesso, documentos, decimal, assinatura, cliente da SEFAZ, DANFE.

- `pkg/` **nunca** importa `internal/` nem `shared/`. Ele não sabe o que é tenant, banco,
  HTTP de entrada ou log.
- `pkg/` tem **API estável**: depois da 1.0, mudança incompatível só em versão maior.
  Quem importa a biblioteca não pode acordar com o build quebrado.
- Dependência externa em `pkg/` precisa de justificativa escrita no PR. Hoje, a única
  aceita é a leitura de PKCS#12.

### 3.2-A Nomes do leiaute

As structs de `pkg/nfe` (e dos próximos leiautes) levam o nome do elemento do XML —
`VProd`, `CClassTrib`, `PIBSUF` —, e não uma tradução para o inglês. É a única exceção à
regra de identificadores em inglês: quem confere uma nota contra o Manual de Orientação do
Contribuinte procura `vProd`, e um `ProductValue` obrigaria a uma tabela de tradução que
ninguém mantém.

E os testes validam todo XML gerado contra o **XSD oficial** (`pkg/nfe/testdata/schemas`).
O CI instala o `xmllint` para que essa validação nunca seja pulada.

### 3.3 Três modelos, nunca um

`entity.Issuer` ≠ `model.IssuerModel` ≠ `response.IssuerResponse`, com mapper entre eles.

### 3.4 Handler fino

Lê a requisição, mapeia para command/query, chama o use case, traduz o erro, escreve a
resposta. Zero regra de negócio, zero SQL, zero cálculo de tributo.

---

## Artigo 4 — Persistência

1. **SQL puro.** Proibido ORM e query builder. Só `database/sql` com placeholders `$1`.
2. **UUID v7 gerado no Go**, nunca pelo banco — mesmo com o `uuidv7()` nativo do
   Postgres 18. O ID precisa existir antes do INSERT para entidade e evento irem na mesma
   transação.
3. **Models sem tags.** O scan é explícito.
4. **Migrations por módulo**, numeradas, em
   `infrastructure/persistence/postgres/migrations/`, registradas em `cmd/api/migrate.go`.
5. **Prefixo de tabela** com o nome do módulo: `issuer_issuers`, `invoice_invoices`.
6. **Postgres 18** é a versão suportada — a mesma em desenvolvimento, teste e na imagem
   de referência do `docker-compose.yml`.

---

## Artigo 5 — Números fiscais

### 5.1 Nenhum `float64`, em camada nenhuma

Valor, quantidade, alíquota e percentual de redução são **ponto fixo inteiro**
(`pkg/decimal`). É proibido `float64` em domínio, banco (`NUMERIC` ou `BIGINT`, nunca
`REAL`/`DOUBLE`), JSON e XML. A serialização para o XML parte do inteiro.

Motivo: a SEFAZ recalcula os totais e rejeita a nota por um centavo (rejeição 531 a 535 e
as equivalentes do IBS/CBS). Com float, a divergência aparece só em alguns valores — os
piores bugs são os que não se reproduzem.

### 5.2 A escala é a do leiaute

Cada campo usa a escala que o Manual de Orientação do Contribuinte define: `vProd` com 2
casas, `qCom` com até 4, `vUnCom` com até 10, alíquotas com 4. O arredondamento é feito
**uma vez**, no ponto que o manual define, e com a regra do manual.

### 5.3 Totais fecham na conta

O total da nota é a soma dos itens já arredondados — nunca o arredondamento da soma. Há
teste de invariante para isso.

---

## Artigo 6 — A nota fiscal

### 6.1 Nenhuma nota sai duas vezes

- Toda emissão exige `Idempotency-Key`. A mesma chave devolve a mesma nota.
- A **chave de acesso é gerada e gravada antes do envio**.
- Timeout, queda de conexão ou resposta ilegível da SEFAZ **nunca** levam a reenvio às
  cegas: primeiro se **consulta pela chave**. Só se reenvia o que a SEFAZ diz não
  conhecer.

### 6.2 A numeração é do banco

Série e próximo número são reservados com `SELECT … FOR UPDATE`. Número reservado e não
usado é **inutilizado**, nunca reaproveitado em silêncio nem esquecido.

### 6.3 O XML autorizado é imutável

O XML assinado e o protocolo (`procNFe`) são gravados como recebidos, com hash SHA-256.
Cancelamento, carta de correção e qualquer evento são **registros novos**. Nada fiscal é
apagado nem editado. O prazo de guarda é de **5 anos** e é configurável só para cima.

### 6.4 O ambiente padrão é homologação

Emitente novo nasce em `homologation`. Passar para `production` é um caso de uso próprio,
que confere o cadastro inteiro. Nota de produção emitida por engano custa um cancelamento;
o padrão tem de ser o erro barato.

---

## Artigo 7 — Paginação

Nenhuma lista sem `LIMIT`, em nenhuma camada. Use `shared/pagination`. Teto de 100 por
página; listas de notas usam cursor quando passarem de milhares.

---

## Artigo 8 — Tenant

O Kang Fiscal serve **vários emitentes** num mesmo deploy (o ERP que emite para os
clientes dele, o contador com a carteira inteira). Toda tabela de negócio tem
`tenant_id`, e o tenant **vem da credencial da requisição, nunca do corpo** — cliente que
escolhe o próprio tenant emite nota no CNPJ do vizinho.

---

## Artigo 9 — Segurança

1. **Certificado A1.** O `.pfx` e a senha assinam qualquer documento em nome da empresa.
   A chave privada é cifrada em repouso com AES-256-GCM (`shared/secretbox`), com a chave
   mestra **só no ambiente**. Ela é decifrada em memória no momento de assinar, nunca é
   logada, nunca volta numa resposta, nunca vai para evento ou fila. Todo tipo que a
   carrega implementa `String()` e `GoString()` devolvendo `[certificado omitido]`.
2. **mTLS com a SEFAZ** valida a cadeia ICP-Brasil. `InsecureSkipVerify` é proibido.
3. **Credenciais de API** são mostradas uma única vez. No banco fica só o HMAC-SHA256 com
   pepper do ambiente. A comparação é em tempo constante.
4. **Autorização no use case**, não só na rota: uma rota nova esquecida no middleware não
   pode virar vazamento.
5. **XML de fora é hostil.** Parser sem entidades externas, com teto de tamanho e de
   profundidade.
6. Todo input passa por value object antes de chegar ao domínio.
7. Rate limit por credencial e por IP.
8. A imagem roda como usuário sem privilégio, com binário estático e sem shell de
   depuração.

---

## Artigo 10 — Erros, logs e observabilidade

- Erro de domínio é `var Err… = errors.New(…)` em `domain/error.go`, checado com
  `errors.Is`.
- O handler traduz erro → status num ponto único por módulo. A API nunca vaza erro de
  banco.
- **Rejeição da SEFAZ não é erro interno**: volta ao cliente com `cStat` e `xMotivo`
  originais, porque é isso que o contador vai pesquisar.
- Log estruturado (Zap) com `req_id` e `tenant_id`. Nunca PII, certificado ou segredo.
- `/health` (liveness), `/ready` (Postgres) e `/metrics` (Prometheus).

---

## Artigo 11 — Testes

Obrigatórios:

1. **Value objects e `pkg/`**: caminho válido e inválidos. Documento, chave de acesso e
   decimal têm teste com os exemplos dos manuais oficiais.
2. **Fronteira de módulo**: `architecture_test.go` e `sql_boundary_test.go`.
3. **Golden XML**: para cada cenário fiscal, o XML gerado é comparado byte a byte com um
   arquivo de referência conferido contra a SEFAZ de homologação.
4. **Autorização**: cada rota protegida prova que credencial errada recebe 401/403.
5. Integração sempre num **Postgres descartável** (`make check`), nunca no de dev.

---

## Artigo 12 — Alíquota é dado, nunca código

CBS, IBS (UF e município), IS, `cClassTrib`, CFOP, as URLs da SEFAZ e o calendário da
transição (2026–2033) vivem em **tabelas versionadas por vigência** (`valid_from`,
`valid_to`), atualizáveis por importação, sem deploy.

Motivo: as alíquotas definitivas da reforma tributária ainda dependem de resolução do
Senado, e as notas técnicas mudam várias vezes por ano. Um `if ano >= 2029` no código é
uma nota rejeitada esperando a próxima NT.

A nota congela, no snapshot, a alíquota e a versão da tabela que usou. Nota antiga nunca
é recalculada.

---

## Artigo 13 — O que nunca se faz

- `float64` em número fiscal.
- Reenviar nota sem consultar pela chave.
- Editar ou apagar XML autorizado.
- Logar certificado, senha ou CSC.
- Copiar código de projeto GPL/LGPL. Projetos MIT, BSD e Apache-2.0 podem ser
  reaproveitados com crédito no `NOTICE`.
- Alíquota no código.

---

## Artigo 14 — Emenda

Toda emenda registra: (a) a regra mudada, (b) o problema que a motivou, (c) o que se perde.
