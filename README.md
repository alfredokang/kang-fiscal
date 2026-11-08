# Kang Fiscal

**Emissor gratuito e open source de NF-e, NFC-e e NFS-e, pronto para a reforma tributária (IBS, CBS e IS) e para os tributos atuais.**

Feito por **Alfredo Kang** para ajudar empresários e desenvolvedores do Brasil a acelerar
seus projetos.

[![Licença Apache-2.0](https://img.shields.io/badge/licença-Apache--2.0-blue.svg)](LICENSE)
![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8)
![Postgres 18](https://img.shields.io/badge/Postgres-18-336791)

> **Estado:** emissão, cancelamento, contingência, recebimento e NFS-e estão prontos e testados contra
> SEFAZ e Sefin simuladas; falta a validação em homologação real. Veja o [plano](docs/00-plano.md).

---

## Por que o Kang Fiscal

- **Auto-hospedado e gratuito.** Sem mensalidade por nota, sem mandar o certificado da sua
  empresa para o servidor de ninguém.
- **Leve.** Uma imagem de ~20 MB, sem shell, e só o Postgres 18 como dependência.
- **Pronto para a reforma.** IBS, CBS e Imposto Seletivo desde o primeiro dia, com as
  alíquotas em tabela versionada por vigência — porque elas ainda vão mudar.
- **Uma API para qualquer linguagem**, numa imagem Docker, com a documentação navegável
  em `/docs`.
- **Sem `float64`.** Todo valor fiscal é ponto fixo inteiro. A SEFAZ rejeita por um centavo;
  aqui o centavo fecha.
- **Nenhuma nota sai duas vezes.** A chave de acesso é gravada antes do envio, e uma queda
  de conexão vira consulta, nunca reenvio às cegas.

## Começando em 1 minuto

Só com a imagem: siga [docs/docker-hub.md](docs/docker-hub.md).

Com o código:

```bash
git clone https://github.com/alfredokang/kang-fiscal && cd kang-fiscal
cp .env.example .env
# defina ADMIN_TOKEN e SECRETS_ENCRYPTION_KEY no .env (openssl rand -base64 32)
docker compose --profile full up -d
curl localhost:8080/ready
```

Ou só a imagem, apontando para o seu Postgres:

```bash
docker run --rm -e POSTGRES_HOST=... -e POSTGRES_USER=... -e POSTGRES_PASSWORD=... \
  -e POSTGRES_DB=... alfredokang/kang-fiscal migrate
docker run -d -p 8080:8080 -e ADMIN_TOKEN=... -e POSTGRES_HOST=... alfredokang/kang-fiscal
```

## Configuração

Tudo vem de variáveis de ambiente (ou de um `.env` ao lado do binário). O
[`.env.example`](.env.example) traz todas comentadas.

### Obrigatórias

| Variável | Para quê |
|---|---|
| `POSTGRES_HOST`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | O banco. **Só Postgres 18**: a API, o migrate e o worker recusam outra versão ao conectar |
| `ADMIN_TOKEN` | Protege as rotas `/v1/admin`. **Vazio desliga essas rotas** (respondem 503) |
| `SECRETS_ENCRYPTION_KEY` | 32 bytes em base64. Cifra (AES-256-GCM) a chave privada e a senha do certificado A1 no banco. Sem ela, o envio de certificado é recusado |

Gere os segredos com `openssl rand -base64 32`, um diferente para cada variável.

> ⚠️ **Guarde a `SECRETS_ENCRYPTION_KEY` fora do servidor.** Sem ela, os certificados
> gravados no banco não podem ser lidos: o backup do Postgres sozinho não basta, e a única
> saída é enviar os certificados de novo.

### Opcionais

| Variável | Padrão | Para quê |
|---|---|---|
| `API_PORT` | `8080` | Porta HTTP |
| `ENVIRONMENT` | `development` | `production` liga o log em JSON |
| `POSTGRES_PORT` | `5432` | |
| `POSTGRES_MAX_CONNS` | `20` | Conexões por processo (api, worker, migrate). Num Postgres compartilhado com outro sistema, baixe para 10 |
| `POSTGRES_SSL_MODE` | `disable` | `require` ou `verify-full` quando o banco está em outra máquina |
| `CREDENTIAL_PEPPER` | — | Necessária só para criar credenciais de tenant (`kf_...`). Trocar invalida todas |
| `RATE_LIMIT_PER_MINUTE` | `600` | Requisições por credencial por minuto; `0` desliga |
| `RESP_TEC_CNPJ`, `RESP_TEC_CONTACT`, `RESP_TEC_EMAIL`, `RESP_TEC_PHONE` | — | Responsável técnico (`infRespTec`) — a empresa que mantém o emissor. Várias UFs exigem; vazio, o grupo não vai na nota |
| `VOID_SWEEP_MINUTES` | `60` | De quanto em quanto tempo o worker roda |
| `VOID_AFTER_HOURS` | `24` | Depois de quanto tempo um número reservado sem nota é inutilizado |

### Os processos

A mesma imagem roda tudo; o comando escolhe o papel:

| Comando | O que faz |
|---|---|
| `migrate` | Cria e atualiza as tabelas. Rode antes da `api` a cada versão nova |
| `api` | A API HTTP |
| `worker` | Tarefas periódicas: transmite NFC-e offline, encerra contingências, busca notas de fornecedores, resolve notas e cancelamentos sem resposta e inutiliza números sem nota. Pode ter mais de uma réplica: um lock no Postgres garante uma rodada por vez |
| `sefaz-status` | Testa o certificado A1 contra a SEFAZ, sem banco |
| `healthcheck` | Usado pelo `HEALTHCHECK` da imagem |

## Em produção

- **HTTPS na frente.** A API fala HTTP puro; ponha um proxy com TLS (Caddy, Traefik,
  Nginx). O token vai no cabeçalho de cada requisição.
- **Backup do Postgres** todo dia, e a `SECRETS_ENCRYPTION_KEY` guardada à parte (veja acima).
- **O `worker` rodando sempre.** Sem ele, NFC-e offline não é transmitida (prazo de 24 horas)
  e nota sem resposta fica esperando você repetir a chamada.
- **Homologação primeiro.** O emitente nasce em homologação; só passe para produção
  (`POST /v1/admin/issuers/{id}/production`) depois de emitir, cancelar e imprimir lá.
- **Relógio certo e UTC no servidor.** A SEFAZ recusa nota com horário no futuro, e o prazo
  de cancelamento conta horas desde a emissão.
- **Docker Swarm com Portainer:** as stacks prontas (banco, emissor e backup) e o passo a
  passo estão em [docs/03-deploy-swarm.md](docs/03-deploy-swarm.md).
- **Fixe a versão da imagem** (`alfredokang/kang-fiscal:<versão>`) em vez de `latest`, e rode
  o `migrate` a cada atualização.

## Primeiros passos

### Cadastrar o emitente

```bash
curl -X POST localhost:8080/v1/admin/issuers \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "cnpj": "12.ABC.345/01DE-35",
    "legal_name": "Minha Empresa LTDA",
    "tax_regime": 1,
    "state_registration": "ISENTO",
    "address": { "street": "Rua das Flores", "number": "100", "district": "Centro",
                 "city_code": "3550308", "city_name": "São Paulo", "uf": "SP",
                 "zip_code": "01001-000" }
  }'
```

A resposta diz se o cadastro já pode emitir (`can_issue`) e, se não, **o que falta**
(`missing`). Isso evita descobrir pela rejeição da SEFAZ, um campo de cada vez. O emitente
nasce em **homologação**, e a passagem para produção é uma chamada separada:
`POST /v1/admin/issuers/{id}/production`.

### Vários clientes na mesma instalação (ERP, contador)

Com o `ADMIN_TOKEN` você administra a instalação e opera o tenant `default` em
`/v1/admin/...`. Para cada cliente, crie uma credencial — o token aparece **uma única vez**:

```bash
curl -X POST localhost:8080/v1/admin/credentials -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"tenant_id": "cliente-1", "name": "ERP do cliente 1", "scopes": ["setup", "nfe:write", "nfe:read"]}'
# { "token": "kf_ab12cd34_...", ... }
```

O cliente usa as mesmas rotas sem `/admin` — `/v1/issuers`, `/v1/tax-profiles`, `/v1/nfe` —
e só enxerga os próprios dados.

| Escopo | Alcança |
|---|---|
| `setup` | emitentes, certificados, séries e perfis tributários |
| `nfe:write` | emitir, cancelar, corrigir e inutilizar |
| `nfe:read` | consultar notas, baixar XML e DANFE, simular tributos |

As alíquotas da reforma são da instalação inteira e só o `ADMIN_TOKEN` cadastra.

### Emitir a primeira NF-e em homologação

Com o emitente cadastrado (e completo), o certificado enviado e um perfil tributário:

```bash
curl -X POST localhost:8080/v1/nfe -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Idempotency-Key: pedido-1001" -d '{
  "issuer_id": "<id do emitente>", "nature_of_operation": "VENDA DE MERCADORIA",
  "recipient": { "cnpj": "99888777000166", "name": "CLIENTE LTDA", "ie_indicator": 9,
    "address": { "street": "AV PAULISTA", "number": "1000", "district": "BELA VISTA",
                 "city_code": "3550308", "city_name": "SAO PAULO", "uf": "SP", "zip_code": "01310100" } },
  "items": [{ "code": "CAFE-500", "description": "CAFE TORRADO 500G", "ncm": "09012100",
              "unit": "UN", "quantity": "3", "unit_price": "33.33", "tax_profile": "venda-sp" }],
  "payments": [{ "method": "17", "amount": "99.99" }] }'
```

| Resposta | Significa |
|---|---|
| `201` `authorized` | Autorizada. O XML de distribuição está em `GET /v1/nfe/{chave}/xml` |
| `422` `rejected` / `denied` | A SEFAZ recusou; `cstat`, `motive` e `hint` dizem o porquê e o que corrigir |
| `202` `uncertain` | A SEFAZ não respondeu de forma conclusiva. Repita a MESMA chamada (mesma `Idempotency-Key`) ou consulte `GET /v1/nfe/{chave}?sync=sefaz` — a nota nunca sai duas vezes |
| `502` `not_sent` | A SEFAZ recusou o certificado na conexão; nada foi processado |

### Cancelar e corrigir

```bash
curl -X POST localhost:8080/v1/nfe/$CHAVE/cancel -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"justification": "Cliente desistiu da compra antes da entrega"}'

# Cada carta de correção SUBSTITUI as anteriores: o texto traz todas as correções da nota.
curl -X POST localhost:8080/v1/nfe/$CHAVE/correction -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"text": "Complemento do endereco do destinatario: SALA 2"}'

curl localhost:8080/v1/nfe/$CHAVE/events -H "Authorization: Bearer $ADMIN_TOKEN"
```

Repetir um cancelamento é seguro: o registrado é devolvido como está, e o que ficou sem
resposta é reenviado — se já tinha chegado, a SEFAZ responde duplicidade e ele conta como
registrado.

### Emitir uma NFS-e (padrão nacional)

A nota de serviço vai direto para o Sistema Nacional da NFS-e (Sefin), sem prefeitura no
meio. Cadastre a `municipal_registration` do emitente e envie a DPS:

```bash
curl -X POST localhost:8080/v1/nfse -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Idempotency-Key: os-2026-0042" -d '{
  "issuer_id": "'$EMITENTE'", "series": 1,
  "service": {"national_code": "010701", "description": "Suporte técnico em informática"},
  "amount": "1500.00",
  "iss": {"taxation": 1, "retention": 1, "rate": "2"},
  "recipient": {"cnpj": "99888777000166", "name": "CLIENTE LTDA", "city_code": "3550308"},
  "ibs_cbs": {"cst": "000", "class_trib": "000001", "operation_code": "100301"}
}'
```

O Sistema Nacional calcula IBS e CBS e devolve a NFS-e. Se a resposta se perder, o Kang
Fiscal consulta a DPS pelo Id antes de pensar em reenviar: o mesmo serviço nunca vira duas
notas. `GET /v1/nfse/{chave}/xml` baixa a NFS-e e `GET /v1/nfse/{chave}/danfse`, o DANFSe em PDF.

Para cancelar:

```bash
curl -X POST localhost:8080/v1/nfse/$CHAVE/cancel -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"reason": 1, "justification": "Valor do serviço informado errado"}'
```

Onde o município não deixa cancelar direto (a Sefin responde E0823), use
`"mode": "analysis"`: vai o pedido de análise fiscal, a nota segue válida e o worker a marca
como cancelada quando o município deferir. O E0840 ("já há evento vinculado") nunca é lido
como "cancelada": o Kang Fiscal consulta os eventos da nota antes de decidir.

### Testar o seu certificado A1

```bash
KANG_PFX_PASSWORD='senha-do-a1' docker run --rm -e KANG_PFX_PASSWORD -v $PWD/a1.pfx:/a1.pfx:ro \
  alfredokang/kang-fiscal sefaz-status --pfx /a1.pfx --uf SP
# SEFAZ: 107 — Serviço em Operação
```

A senha vai por variável de ambiente, e não por flag, para não ficar no histórico do shell.

### Simular os tributos (o carrinho mostra o que vai para a nota)

```bash
curl -X PUT localhost:8080/v1/admin/tax-profiles/venda-sp -H "Authorization: Bearer $ADMIN_TOKEN" -d '{
  "name": "Venda dentro de SP",
  "rules": { "cfop_internal": "5102", "cfop_interstate": "6102",
             "icms": {"code": "00", "rate": "18"},
             "pis": {"cst": "01", "rate": "1.65"}, "cofins": {"cst": "01", "rate": "7.6"},
             "ibs_cbs": {"cst": "000", "class_trib": "000001"},
             "exclude_icms_from_pis_base": true } }'

curl -X POST localhost:8080/v1/admin/tax/simulate -H "Authorization: Bearer $ADMIN_TOKEN" -d '{
  "uf": "SP", "city_code": "3550308",
  "items": [{ "profile": "venda-sp", "quantity": "3", "unit_price": "33.33" }] }'
# totals: v_icms 18.00, v_pis 1.35, v_cofins 6.23, v_cbs 0.67, v_ibs 0.07, v_nf 99.99
```

As alíquotas de IBS e CBS ficam numa tabela por vigência (`/v1/admin/tax-rates`). Só as de
2026 vêm cadastradas; para datas sem alíquota oficial, a simulação e a emissão respondem
`TAX_RATE_MISSING` em vez de usar um número inventado.

## Usar como biblioteca Go

```go
import (
    "github.com/alfredokang/kang-fiscal/pkg/accesskey"
    "github.com/alfredokang/kang-fiscal/pkg/decimal"
    "github.com/alfredokang/kang-fiscal/pkg/document"
)

cnpj, _ := document.ParseCNPJ("12.ABC.345/01DE-35") // CNPJ alfanumérico (jul/2026)

base, _ := decimal.Parse("33.33", 2)
pCBS, _ := decimal.Parse("0.9", 4)
vCBS, _ := base.Percent(pCBS, 2, decimal.HalfUp) // 0.30 — sem float

code, _ := accesskey.NewCode(123)
key, _ := accesskey.Build(accesskey.Parts{UFCode: 35, Issued: time.Now(),
    Document: string(cnpj), Model: accesskey.NFe, Series: 1, Number: 123,
    EmissionType: 1, Code: code})
```

## O que já existe

| Peça | Situação |
|---|---|
| `pkg/decimal` — ponto fixo com a escala do leiaute, arredondamento explícito | ✅ |
| `pkg/document` — CNPJ (inclusive alfanumérico) e CPF | ✅ |
| `pkg/accesskey` — chave de acesso de 44 posições e cNF aleatório | ✅ |
| Módulo `issuer` — cadastro do emitente, homologação → produção | ✅ |
| Fundação: Postgres 18, migrations, guards de arquitetura, imagem distroless | ✅ |
| `pkg/certificate` e `pkg/xmldsig` — A1 e assinatura, conferida contra NF-e reais | ✅ |
| Módulo `certificate` — A1 cifrado por emitente | ✅ |
| `pkg/sefaz` — cliente das 27 UFs com cadeia ICP-Brasil verificada e 528 códigos de retorno com dica | ✅ |
| `kang-fiscal sefaz-status` — testa o seu A1 contra a SEFAZ | ✅ |
| `pkg/nfe` — leiaute 4.00 com os tributos atuais e IBS/CBS/IS na mesma nota, validado contra o XSD oficial | ✅ |
| Motor tributário (`pkg/tax` + módulo `taxrule`): perfis, alíquotas por vigência e simulação de tributos no carrinho | ✅ |
| Numeração sem buraco nem repetição | ✅ |
| `POST /v1/nfe` — emissão da NF-e com idempotência e consulta pela chave quando a SEFAZ não responde | ✅ |
| Cancelamento e carta de correção (`POST /v1/nfe/{chave}/cancel` e `/correction`) | ✅ |
| DANFE em PDF (`GET /v1/nfe/{chave}/danfe`), sem dependência | ✅ |
| Inutilização manual (`POST /v1/nfe/void-range`) e automática (`kang-fiscal worker`) | ✅ |
| Credenciais por tenant (`kf_...`) com escopos e limite por minuto | ✅ |
| NFC-e (modelo 65) com QR Code v3 e DANFCE em bobina de 80 mm | ✅ |
| Contingência SVC automática da NF-e (e a nota incerta nunca vira duas) | ✅ |
| NFC-e offline: o caixa não para quando a SEFAZ cai, e a venda nunca vira duas notas | ✅ |
| Recebimento de notas de fornecedores e manifestação do destinatário (`/v1/dfe/...`) | ✅ |
| NFS-e no padrão nacional (`POST /v1/nfse`), com IBS/CBS, sem duplicar nota em timeout | ✅ |
| Cancelamento da NFS-e, direto ou por análise fiscal (`POST /v1/nfse/{chave}/cancel`) | ✅ |
| DANFSe v2.0 em PDF (`GET /v1/nfse/{chave}/danfse`), gerado localmente — a API de PDF do governo acabou em 01/07/2026 | ✅ |
| Especificação OpenAPI 3.1 (`/openapi.yaml`) e documentação navegável (`/docs`), conferidas contra as rotas reais a cada build | ✅ |
| SDKs (TS, PHP, Python) gerados da especificação | 🔜 F6 |

## Desempenho

`make load` simula vendas em volume: a API num servidor HTTP real, o Postgres 18 com o pool
da produção (20 conexões) e uma SEFAZ simulada. No fim, confere no banco e na "SEFAZ" que
nenhuma venda virou duas notas e que a numeração não tem buraco nem repetição. Numa máquina
de desenvolvimento (Apple Silicon, Postgres no Docker local), uma instância só:

| Cenário | Vazão | Latência (p50 / p99) |
|---|---|---|
| SEFAZ instantânea — o teto do próprio Kang Fiscal | ~250 a 440 notas/s | 0,2 s / 0,5 s |
| SEFAZ realista (200 a 800 ms por nota), 200 clientes simultâneos | ~190 a 330 notas/s | 0,6 s / 1,0 s |
| Caos: 10% das respostas da SEFAZ se perdem e 5% das vendas são reenviadas ao mesmo tempo | ~170 a 260 notas/s | 0,6 s / 1,5 s |

Nos três, **zero nota duplicada e zero buraco na numeração**. São números de uma máquina, não
uma promessa: o limite real em produção é quanto a SEFAZ da sua UF aceita. A API não guarda
estado, então dá para rodar várias réplicas; para volume alto de NFC-e, use uma série por
caixa.

## Documentação da API

Com o Kang Fiscal rodando, abra `http://localhost:8080/docs`. A especificação está em
`/openapi.yaml` (e em `shared/openapi/openapi.yaml`): dá para gerar cliente em qualquer
linguagem com o openapi-generator.

## Arquitetura

O Kang Fiscal é um **monolito modular** em Go: um binário, com fronteiras de microsserviço
entre os módulos, vigiadas por testes que quebram o build. As regras estão na
[Constituição](CONSTITUTION.md), e quem for contribuir deve começar por ela.

```
pkg/        a biblioteca fiscal — não conhece banco, HTTP nem tenant
internal/   os módulos da API (domain → application → infrastructure / adapter)
shared/     infraestrutura comum: config, log, Postgres, paginação, cifra, servidor, OpenAPI
cmd/        a CLI: api, migrate, worker, sefaz-status, healthcheck
```

## Desenvolvimento

```bash
make setup   # .env + dependências
make dev     # sobe o Postgres do compose + migrations + API
make local   # usa um Postgres 18 que você já tem rodando (POSTGRES_* do .env)
make test    # testes sem banco
make check   # gofmt, vet e a suíte inteira contra um Postgres 18 descartável
```

## Contribuindo

Contribuições são bem-vindas. Leia o [CONTRIBUTING.md](CONTRIBUTING.md) e a
[Constituição](CONSTITUTION.md) antes de abrir um pull request. Encontrou uma falha de
segurança? **Não abra issue pública** — veja o [SECURITY.md](SECURITY.md).

## Licença e créditos

O Kang Fiscal é distribuído sob a [Licença Apache 2.0](LICENSE): pode usar, modificar e
vender, inclusive em produto fechado, desde que mantenha o aviso de copyright e o
[NOTICE](NOTICE). Copyright 2026 Alfredo Kang.

Ele se apoia no trabalho de outros projetos, todos com licença compatível:

| O quê | De onde | Licença |
|---|---|---|
| Gerador de PDF do DANFE e leitura do A1 (adaptados) | [mschunke/gonfe](https://github.com/mschunke/gonfe) | MIT |
| Endereços das SEFAZ e catálogo dos códigos de retorno | [nfephp-org/sped-nfe](https://github.com/nfephp-org/sped-nfe) | MIT |
| Cadeia de certificados ICP-Brasil | [rffontenelle/CertificadosAC-ICP-Brasil](https://github.com/rffontenelle/CertificadosAC-ICP-Brasil) (certificados públicos do ITI) | CC0 |
| Schemas da DPS da NFS-e Nacional (testes) | [Gomes343/PyNFSeNacional](https://github.com/Gomes343/PyNFSeNacional) | MIT |
| Schemas do evento da NFS-e (testes) | [akretion/nfelib](https://github.com/akretion/nfelib) | MIT |
| Leitura do `.pfx` | [SSLMate/go-pkcs12](https://github.com/SSLMate/go-pkcs12) | BSD-3 |
| QR Code | [skip2/go-qrcode](https://github.com/skip2/go-qrcode) | MIT |
| Leitura do QR nos testes | [makiuchi-d/gozxing](https://github.com/makiuchi-d/gozxing) | Apache-2.0 |
| Tabela de municípios | API de localidades do [IBGE](https://servicodados.ibge.gov.br/api/docs/localidades) | dado público |
| Schemas da NF-e e da NFS-e | Portal da NF-e e SE/CGNFS-e (Receita Federal) | documento público |

Projetos com licença LGPL/GPL (como o sped-da e a BrazilFiscalReport) foram só estudados,
nunca copiados. A lista completa, com o que cada um ensinou, está em
[docs/01-referencias.md](docs/01-referencias.md).

> **Aviso.** O Kang Fiscal é uma ferramenta, fornecida "como está", sem garantia (seções 7 e
> 8 da licença). A responsabilidade pela emissão, pelo enquadramento tributário e pelas
> alíquotas usadas é do emitente e do contador dele. Teste sempre em homologação antes de
> emitir em produção.
