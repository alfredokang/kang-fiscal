# Kang Fiscal

**Emissor gratuito e open source de NF-e, NFC-e e NFS-e, pronto para a reforma tributária (IBS, CBS e IS)
e para os tributos atuais.**

Feito por **Alfredo Kang** para ajudar empresários e desenvolvedores do Brasil a acelerar
seus projetos.

- **Na sua máquina, de graça.** Sem mensalidade por nota, e o certificado A1 da sua empresa
  não sai do seu servidor (fica cifrado no seu Postgres).
- **Leve.** Imagem de ~20 MB, sem shell; só precisa de Postgres.
- **NF-e, NFC-e e NFS-e Nacional** — emissão, cancelamento, carta de correção,
  inutilização, DANFE/DANFCE/DANFSe em PDF, contingência (SVC e NFC-e offline) e
  recebimento das notas de fornecedores com manifestação.
- **Reforma tributária:** IBS, CBS e IS na mesma nota que ICMS, PIS e COFINS, com as
  alíquotas em tabela versionada por vigência.
- **Nenhuma nota sai duas vezes.** A chave é gravada antes do envio; sem resposta da
  SEFAZ, a nota é consultada — nunca reenviada às cegas.

## Começando

Salve como `docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_USER: kangfiscal
      POSTGRES_PASSWORD: troque-esta-senha
      POSTGRES_DB: kangfiscal
      TZ: UTC
    volumes:
      - postgres_data:/var/lib/postgresql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U kangfiscal"]
      interval: 5s
      retries: 10

  migrate:
    image: alfredokang/kang-fiscal:latest
    command: ["migrate"]
    env_file: .env
    depends_on:
      postgres: { condition: service_healthy }

  api:
    image: alfredokang/kang-fiscal:latest
    command: ["api"]
    env_file: .env
    ports: ["8080:8080"]
    depends_on:
      migrate: { condition: service_completed_successfully }
    restart: unless-stopped

  # Inutiliza números sem nota, transmite as NFC-e offline e resolve as notas sem resposta.
  worker:
    image: alfredokang/kang-fiscal:latest
    command: ["worker"]
    env_file: .env
    depends_on:
      migrate: { condition: service_completed_successfully }
    restart: unless-stopped

volumes:
  postgres_data:
```

E um `.env` ao lado:

```bash
POSTGRES_HOST=postgres
POSTGRES_USER=kangfiscal
POSTGRES_PASSWORD=troque-esta-senha
POSTGRES_DB=kangfiscal
# Gere cada um com: openssl rand -base64 32
ADMIN_TOKEN=
SECRETS_ENCRYPTION_KEY=
CREDENTIAL_PEPPER=
```

```bash
docker compose up -d
curl localhost:8080/ready
```

Abra **http://localhost:8080/docs**: lá está a API inteira, com exemplos.

## Primeiros passos

1. `POST /v1/admin/issuers` — cadastre a empresa (nasce em **homologação**).
2. `PUT /v1/admin/issuers/{id}/certificate` — envie o certificado A1 (`.pfx` em base64).
3. `PUT /v1/admin/tax-profiles/{code}` — diga como seus produtos são tributados.
4. `POST /v1/nfe` (ou `/v1/nfse`) com o cabeçalho `Idempotency-Key` — emita.
5. Testou tudo em homologação? `POST /v1/admin/issuers/{id}/production`.

Para testar o certificado contra a SEFAZ antes de tudo:

```bash
KANG_PFX_PASSWORD='senha-do-a1' docker run --rm -e KANG_PFX_PASSWORD \
  -v $PWD/a1.pfx:/a1.pfx:ro alfredokang/kang-fiscal sefaz-status --pfx /a1.pfx --uf SP
```

## Código e licença

Código em [github.com/alfredokang/kang-fiscal](https://github.com/alfredokang/kang-fiscal).
Apache-2.0: uso comercial liberado.
