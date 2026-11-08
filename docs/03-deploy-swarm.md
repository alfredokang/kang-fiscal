# Deploy no Docker Swarm com Portainer

Três stacks, nesta ordem:

| Stack no Portainer | Arquivo | O que roda |
|---|---|---|
| `postgres_kangfiscal` | `deploy/stack-postgres-kangfiscal.yml` | Postgres 18 |
| `kangfiscal` | `deploy/stack-kangfiscal.yml` | API, worker e migrate |
| `backup_kangfiscal` | `deploy/stack-backup-postgres-kangfiscal.yml` | Backup diário conferido |

Pressupõe o que o servidor já tem: Swarm iniciado, a rede `network_public` e o Traefik com o
resolvedor `letsencryptresolver` (o mesmo da Minas Café).

## 1. Antes de tudo

**DNS:** crie um registro `A` do domínio da API (ex.: `fiscal.seudominio.com.br`) apontando
para o IP do servidor. O Traefik só emite o HTTPS depois que o DNS responde.

**No servidor (nó manager), uma vez:**

```bash
docker volume create kangfiscal_postgres_data
mkdir -p /root/backups/kangfiscal
```

**As variáveis:** copie `deploy/kangfiscal.env.example`, preencha e guarde num lugar seguro
(gerenciador de senhas). Para gerar os segredos:

```bash
for v in ADMIN_TOKEN SECRETS_ENCRYPTION_KEY CREDENTIAL_PEPPER; do echo "$v=$(openssl rand -base64 32)"; done
```

> ⚠️ A `SECRETS_ENCRYPTION_KEY` cifra os certificados A1 no banco e **não vai no backup**.
> Sem ela, nem o backup devolve os certificados. Guarde uma cópia fora do servidor.

## 2. O banco — `postgres_kangfiscal`

Portainer → **Stacks → Add stack**:

1. Nome: `postgres_kangfiscal`
2. Web editor: cole `deploy/stack-postgres-kangfiscal.yml`
3. Environment variables: `POSTGRES_PASSWORD`
4. **Deploy the stack**

Espere o serviço `postgres_kangfiscal_postgres` ficar `running` (healthy).

## 3. O emissor — `kangfiscal`

1. Nome: `kangfiscal`
2. Web editor: cole `deploy/stack-kangfiscal.yml`
3. Environment variables → **Load variables from .env file** → o `kangfiscal.env` preenchido
4. **Deploy the stack**

O `migrate` cria as tabelas e termina (fica `complete`). A API e o worker sobem em seguida.
Se a API subir antes do migrate terminar e cair, o Swarm a reinicia sozinho.

Confira:

```bash
curl https://fiscal.seudominio.com.br/ready      # {"status":"ready"}
```

E abra `https://fiscal.seudominio.com.br/docs`.

## 4. O backup — `backup_kangfiscal`

1. Nome: `backup_kangfiscal`
2. Web editor: cole `deploy/stack-backup-postgres-kangfiscal.yml`
3. Environment variables: `POSTGRES_PASSWORD` (a mesma do banco)
4. Deploy. Nos logs deve aparecer `backup: ok`, e o arquivo em
   `/root/backups/kangfiscal/diario/`.

Rodízio: 7 diários, 5 semanais e 12 mensais. Os XMLs têm de ser guardados por 5 anos: o
banco é quem guarda; o backup é para quando o servidor se perde.

**A cópia fora do servidor é obrigatória** — backup que mora só no servidor morre com ele.
No Mac, todo dia às 9h (`crontab -e`):

```
0 9 * * * rsync -az root@IP_DO_SERVIDOR:/root/backups/kangfiscal/ ~/Backups/kangfiscal/
```

## 5. Primeiros passos com a API no ar

```bash
API=https://fiscal.seudominio.com.br
TOKEN=<ADMIN_TOKEN>

# 1. O emitente (nasce em homologação)
curl -X POST $API/v1/admin/issuers -H "Authorization: Bearer $TOKEN" -d '{ ... }'

# 2. O certificado A1 (no Linux, troque `base64 -i a1.pfx` por `base64 -w0 a1.pfx`)
curl -X PUT $API/v1/admin/issuers/<id>/certificate -H "Authorization: Bearer $TOKEN" \
  -d "{\"pfx_base64\": \"$(base64 -i a1.pfx)\", \"password\": \"<senha do A1>\"}"
```

Os exemplos completos estão no README e em `/docs`.

## Atualizar

1. Na máquina de desenvolvimento: commit, `git push` e `make publish`.
2. Portainer → Stacks → `kangfiscal` → **Update the stack** → marque **Re-pull image and
   redeploy**.

O `migrate` roda de novo (só aplica o que falta) e a API troca sem parar: a nova sobe e
passa no healthcheck antes da antiga cair (`start-first`).

**Voltar a uma versão anterior:** defina `IMAGE_TAG` com o SHA da versão boa (o que o
`make publish` imprimiu, ex.: `a68e70e`) e atualize a stack. Migrations não voltam: se a
versão nova criou coluna, a velha convive com ela.

## Restaurar o backup

**Teste sempre num banco descartável primeiro.**

```bash
# no servidor: copie o dump para dentro do contêiner do banco e restaure
docker cp /root/backups/kangfiscal/diario/kangfiscal-AAAA-MM-DD.dump $(docker ps -q -f name=postgres_kangfiscal_postgres):/tmp/r.dump
docker exec -it $(docker ps -q -f name=postgres_kangfiscal_postgres) \
  pg_restore -U kangfiscal -d kangfiscal --clean --if-exists /tmp/r.dump
```

Depois, a API precisa da **mesma** `SECRETS_ENCRYPTION_KEY` de antes para abrir os
certificados.

## Armadilhas

- **Worker reiniciando sem parar:** ele não pode herdar o healthcheck da imagem (que
  consulta a porta da API). A stack já desliga; não remova o `healthcheck: disable: true`.
- **`too many clients`:** o Postgres está com `max_connections=100`. Com mais réplicas da
  API, cada uma abre até 20 conexões: aumente junto.
- **A API recusa o banco** ("só funciona com Postgres 18"): a imagem do banco tem de ser
  `postgres:18-alpine`.
- **HTTPS não sai:** o DNS ainda não propagou, ou a porta 80/443 está fechada no firewall.
- **Porta 5435 aberta para a internet:** o Docker passa por cima do `ufw`. Só deixe o bloco
  `ports` na stack do banco se houver firewall de fora do servidor (Cloud Firewall).
