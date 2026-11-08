-- As credenciais de API de cada tenant. Do segredo, só o HMAC com o pepper do ambiente
-- (Constituição, Art. 9.3): quem leva o dump não leva chave que funcione.
CREATE TABLE IF NOT EXISTS credential_keys (
    id           UUID         PRIMARY KEY,
    tenant_id    VARCHAR(64)  NOT NULL,
    name         VARCHAR(120) NOT NULL,
    prefix       CHAR(11)     NOT NULL,
    hash         CHAR(64)     NOT NULL,
    scopes       TEXT         NOT NULL, -- separados por vírgula
    created_at   TIMESTAMPTZ  NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    CONSTRAINT credential_keys_prefix_key UNIQUE (prefix)
);

CREATE INDEX IF NOT EXISTS credential_keys_tenant_idx ON credential_keys (tenant_id, created_at DESC);
