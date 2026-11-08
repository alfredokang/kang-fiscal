# ─── build ───────────────────────────────────────────────────────────────────
# BUILDPLATFORM: o compilador roda nativo na máquina de quem builda e gera o binário para
# TARGETARCH. Sem isso o buildx emularia a outra arquitetura e o build multi-arch levaria
# dez vezes mais.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /app

# go.mod e go.sum primeiro: a camada de dependências só é reconstruída quando elas mudam.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO desligado: binário estático, que roda numa imagem sem libc e sem shell.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X github.com/alfredokang/kang-fiscal/cmd/api.Version=${VERSION}" \
    -o /out/kang-fiscal . && \
    # Só os .sql vão para a imagem: `kang-fiscal migrate` precisa deles, o código-fonte não.
    tar cf - $(find internal -name '*.sql') | tar xf - -C /out

# ─── runtime ─────────────────────────────────────────────────────────────────
# distroless/static: sem shell, sem gerenciador de pacotes, com os certificados raiz e o
# tzdata. Quem invadir o processo não encontra ferramenta nenhuma (Constituição, Art. 9.8).
# `nonroot` roda como o usuário 65532.
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="Kang Fiscal" \
      org.opencontainers.image.description="Emissor de NF-e, NFC-e e NFS-e open source, pronto para IBS, CBS e IS" \
      org.opencontainers.image.authors="Alfredo Kang" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.url="https://hub.docker.com/r/alfredokang/kang-fiscal"

WORKDIR /app
COPY --from=builder /out/ /app/

EXPOSE 8080

# A imagem não tem wget nem curl: o próprio binário confere o /health.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
    CMD ["/app/kang-fiscal", "healthcheck"]

ENTRYPOINT ["/app/kang-fiscal"]
CMD ["api"]
