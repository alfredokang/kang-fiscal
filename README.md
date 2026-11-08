# Kang Fiscal

**Emissor de NF-e, NFC-e e NFS-e open source, pronto para a reforma tributária (IBS, CBS e IS).**

Feito por **Alfredo Kang** para ajudar empresários e desenvolvedores do Brasil a acelerar
seus projetos.

[![Licença Apache-2.0](https://img.shields.io/badge/licença-Apache--2.0-blue.svg)](LICENSE)
![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8)
![Postgres 18](https://img.shields.io/badge/Postgres-18-336791)

> ⚠️ **Em construção — fase F0.** A fundação, o cadastro do emitente e as primeiras peças da
> biblioteca fiscal estão prontos; a emissão ainda não. Veja o [plano](docs/00-plano.md).

---

## Por que o Kang Fiscal

- **Auto-hospedado e gratuito.** Sem mensalidade por nota, sem mandar o certificado da sua
  empresa para o servidor de ninguém.
- **Leve.** Uma imagem de ~20 MB, sem shell, e só o Postgres como dependência.
- **Pronto para a reforma.** IBS, CBS e Imposto Seletivo desde o primeiro dia, com as
  alíquotas em tabela versionada por vigência — porque elas ainda vão mudar.
- **Duas formas de usar:** a **API** em Docker, para qualquer linguagem, ou a **biblioteca
  Go** (`pkg/`), para quem quer embutir a emissão sem subir outro serviço.
- **Sem `float64`.** Todo valor fiscal é ponto fixo inteiro. A SEFAZ rejeita por um centavo;
  aqui o centavo fecha.
- **Nenhuma nota sai duas vezes.** A chave de acesso é gravada antes do envio, e uma queda
  de conexão vira consulta, nunca reenvio às cegas.

## Começando em 1 minuto

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
| NFC-e, credenciais por tenant | 🔜 |
| NFC-e 65, contingência, DistribuiçãoDF-e, NFS-e Nacional | 🔜 F2–F5 |

## Arquitetura

O Kang Fiscal é um **monolito modular** em Go: um binário, com fronteiras de microsserviço
entre os módulos, vigiadas por testes que quebram o build. As regras estão na
[Constituição](CONSTITUTION.md), e quem for contribuir deve começar por ela.

```
pkg/        a biblioteca fiscal pública — não conhece banco, HTTP nem tenant
internal/   os módulos da API (domain → application → infrastructure / adapter)
shared/     infraestrutura comum: config, log, Postgres, paginação, cifra, servidor
cmd/        a CLI: api, migrate, healthcheck
```

## Desenvolvimento

```bash
make setup   # .env + dependências
make dev     # Postgres + migrations + API
make test    # testes sem banco
make check   # gofmt, vet e a suíte inteira contra um Postgres 18 descartável
```

## Licença

[Apache-2.0](LICENSE). Use em projetos comerciais à vontade; mantenha o [NOTICE](NOTICE).

As referências e os projetos que inspiraram este estão em
[docs/01-referencias.md](docs/01-referencias.md).

> O Kang Fiscal é uma ferramenta. A responsabilidade pela emissão, pelo enquadramento
> tributário e pelas alíquotas usadas é do emitente e do contador dele. Teste sempre em
> homologação antes de emitir em produção.
