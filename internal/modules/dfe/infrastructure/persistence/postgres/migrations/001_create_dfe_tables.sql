-- O recebimento de notas: a posição da distribuição de cada emitente (o NSU) e os
-- documentos que chegaram contra o CNPJ dele.
CREATE TABLE IF NOT EXISTS dfe_cursors (
    tenant_id     VARCHAR(64) NOT NULL,
    issuer_ref    UUID        NOT NULL, -- texto opaco ao issuer, sem FK (Art. 2)
    last_nsu      BIGINT      NOT NULL DEFAULT 0,
    max_nsu       BIGINT      NOT NULL DEFAULT 0,
    -- A SEFAZ bloqueia por uma hora quem consulta sem documento novo: a próxima consulta
    -- só sai a partir daqui.
    next_query_at TIMESTAMPTZ NOT NULL DEFAULT '-infinity',
    last_cstat    INTEGER     NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, issuer_ref)
);

CREATE TABLE IF NOT EXISTS dfe_documents (
    id            UUID         PRIMARY KEY,
    tenant_id     VARCHAR(64)  NOT NULL,
    issuer_ref    UUID         NOT NULL,
    nsu           BIGINT       NOT NULL,
    schema        VARCHAR(40)  NOT NULL,
    kind          VARCHAR(16)  NOT NULL CHECK (kind IN ('summary', 'full', 'event_summary', 'event')),
    access_key    VARCHAR(44)  NOT NULL DEFAULT '',
    emitter_doc   VARCHAR(14)  NOT NULL DEFAULT '',
    emitter_name  VARCHAR(120) NOT NULL DEFAULT '',
    total         VARCHAR(20)  NOT NULL DEFAULT '',
    issued_at     VARCHAR(30)  NOT NULL DEFAULT '',
    situation     VARCHAR(2)   NOT NULL DEFAULT '',
    manifestation VARCHAR(6)   NOT NULL DEFAULT '',
    xml           BYTEA        NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL,
    -- O NSU é único por interessado: a mesma consulta repetida não duplica documento. A
    -- consulta por chave (depois da ciência) traz o NSU original do documento.
    CONSTRAINT dfe_documents_nsu UNIQUE (tenant_id, issuer_ref, nsu, schema)
);

CREATE INDEX IF NOT EXISTS dfe_documents_list_idx ON dfe_documents (tenant_id, issuer_ref, created_at DESC);
CREATE INDEX IF NOT EXISTS dfe_documents_key_idx ON dfe_documents (tenant_id, access_key);
