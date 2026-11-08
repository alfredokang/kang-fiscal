#!/usr/bin/env bash
#
# Roda a suíte inteira — inclusive os testes que exigem Postgres — contra um banco
# DESCARTÁVEL, criado aqui e apagado no fim. Nunca contra o banco de desenvolvimento: os
# testes apagam e gravam emitentes, e a tabela de notas será append-only.
set -euo pipefail

PORT="${INTEGRATION_PG_PORT:-55433}"
NAME="kangfiscal-integration-$$"
# A mesma versão suportada (Constituição, Art. 4.6).
IMAGE="${INTEGRATION_PG_IMAGE:-postgres:18-alpine}"

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "Docker não está rodando: os testes de integração precisam dele." >&2
  exit 1
fi

docker run -d --rm --name "$NAME" \
  -e POSTGRES_USER=integration -e POSTGRES_PASSWORD=integration -e POSTGRES_DB=kangfiscal_integration \
  -p "127.0.0.1:${PORT}:5432" --tmpfs /var/lib/postgresql "$IMAGE" >/dev/null

for _ in $(seq 1 60); do
  # pg_isready responde "aceitando" ainda durante o initdb; a consulta só passa no banco final.
  if docker exec "$NAME" psql -U integration -d kangfiscal_integration -Atc "select 1" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

# `env -i`: só as variáveis deste banco chegam ao migrate. Nada herdado do shell pode
# desviar as migrations para o banco de desenvolvimento.
if ! saida=$(env -i PATH="$PATH" HOME="$HOME" \
  GOPATH="$(go env GOPATH)" GOCACHE="$(go env GOCACHE)" GOMODCACHE="$(go env GOMODCACHE)" GOTOOLCHAIN="$(go env GOTOOLCHAIN)" \
  POSTGRES_USER=integration POSTGRES_PASSWORD=integration POSTGRES_DB=kangfiscal_integration \
  POSTGRES_HOST=127.0.0.1 POSTGRES_PORT="$PORT" POSTGRES_SSL_MODE=disable \
  go run . migrate 2>&1); then
  echo "$saida" >&2
  exit 1
fi

INTEGRATION_DB=1 \
INTEGRATION_DSN="postgresql://integration:integration@127.0.0.1:${PORT}/kangfiscal_integration?sslmode=disable&TimeZone=UTC" \
  go test ${TEST_TARGET:-./... -race} -count=1 "$@"
