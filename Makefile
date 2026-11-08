.PHONY: help setup dev run build test check fmt migrate docker-up docker-full docker-down db-shell image publish docker-login

help: ## Lista os alvos disponíveis
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

setup: ## Copia .env.example para .env e baixa as dependências
	@test -f .env || cp .env.example .env
	go mod download

dev: docker-up migrate run ## Sobe o Postgres do compose, aplica as migrations e roda a API

local: migrate run ## Usa o Postgres 18 que você já tem (POSTGRES_* do .env): migrations + API

run: ## Roda a API local
	go run . api

worker: ## Roda o worker local (noutro terminal, junto com a API)
	go run . worker

build: ## Compila o binário
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o kang-fiscal .

test: ## Só os testes sem banco — rápido
	go test ./... -race

load: ## Simulação de carga (sem -race, que distorce a medição): vazão, latência e nota duplicada
	TEST_TARGET=./shared/server LOAD_TEST=1 ./scripts/test-integration.sh -run TestCarga -v -timeout 30m

check: ## gofmt, vet e a suíte inteira com Postgres 18 descartável (precisa do Docker)
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...
	./scripts/test-integration.sh

fmt: ## Formata o código
	gofmt -w .

migrate: ## Aplica as migrations pendentes
	go run . migrate

docker-up: ## Sobe só o Postgres
	docker compose up -d postgres

docker-full: ## Sobe Postgres, migrate e API pela imagem
	docker compose --profile full up -d --build

docker-down: ## Derruba os contêineres
	docker compose --profile full down

db-shell: ## Abre o psql no banco
	docker compose exec postgres psql -U kangfiscal -d kangfiscal

# ─── Imagem (Docker Hub) ─────────────────────────────────────────────────────

DOCKER_USER ?= alfredokang
IMAGE       := $(DOCKER_USER)/kang-fiscal
SHA         := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
VERSION     ?= $(SHA)
PLATFORMS   ?= linux/amd64,linux/arm64
# O driver padrão do Docker Desktop não monta várias plataformas de uma vez; este builder
# (driver docker-container) monta. É criado na primeira publicação e reaproveitado.
BUILDER     ?= kangfiscal

docker-login: ## Login no Docker Hub (use um access token, não a senha)
	docker login -u $(DOCKER_USER)

image: ## Build local da imagem, sem publicar
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):dev .

publish: check ## check + build multi-arch + push para o Docker Hub (:latest e :<sha>)
	@# Alteração não commitada faria a imagem :<sha> conter código que não existe no commit.
	@git diff --quiet && git diff --cached --quiet || (echo "há alterações não commitadas" >&2; exit 1)
	@docker buildx inspect $(BUILDER) >/dev/null 2>&1 || docker buildx create --name $(BUILDER) --driver docker-container >/dev/null
	docker buildx build --builder $(BUILDER) --platform $(PLATFORMS) --build-arg VERSION=$(VERSION) \
		-t $(IMAGE):latest -t $(IMAGE):$(SHA) --push .
