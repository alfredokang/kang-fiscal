-- A numeração das notas: uma sequência por emitente, modelo, série e ambiente, e o registro
-- de cada número entregue (Constituição, Art. 6.2).

CREATE TABLE IF NOT EXISTS numbering_series (
    tenant_id   VARCHAR(64) NOT NULL,
    issuer_ref  UUID        NOT NULL, -- texto opaco ao módulo issuer, sem FK (Art. 2)
    model       SMALLINT    NOT NULL CHECK (model IN (55, 65)),
    series      SMALLINT    NOT NULL CHECK (series BETWEEN 0 AND 999),
    environment VARCHAR(16) NOT NULL CHECK (environment IN ('homologation', 'production')),
    next_number INTEGER     NOT NULL CHECK (next_number BETWEEN 1 AND 1000000000),
    PRIMARY KEY (tenant_id, issuer_ref, model, series, environment)
);

CREATE TABLE IF NOT EXISTS numbering_reservations (
    id           UUID        PRIMARY KEY,
    tenant_id    VARCHAR(64) NOT NULL,
    issuer_ref   UUID        NOT NULL,
    model        SMALLINT    NOT NULL,
    series       SMALLINT    NOT NULL,
    environment  VARCHAR(16) NOT NULL,
    number       INTEGER     NOT NULL,
    document_ref VARCHAR(80) NOT NULL,
    status       VARCHAR(10) NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved', 'used', 'voided')),
    reserved_at  TIMESTAMPTZ NOT NULL,
    settled_at   TIMESTAMPTZ,
    -- Número repetido é pior que buraco: a SEFAZ rejeita a segunda nota como duplicidade.
    CONSTRAINT numbering_reservations_unique UNIQUE (tenant_id, issuer_ref, model, series, environment, number)
);

-- A varredura dos buracos lê só as reservas abertas, por idade.
CREATE INDEX IF NOT EXISTS numbering_reservations_open_idx
    ON numbering_reservations (reserved_at) WHERE status = 'reserved';
