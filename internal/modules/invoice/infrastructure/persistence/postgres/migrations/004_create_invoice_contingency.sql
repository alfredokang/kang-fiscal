-- O estado de contingência de cada autorizadora: quando a SEFAZ da UF cai, as notas novas
-- saem pela SEFAZ Virtual de Contingência (SVC-AN ou SVC-RS). Sem tenant: a
-- disponibilidade da SEFAZ é a mesma para todos os emitentes da UF.
CREATE TABLE IF NOT EXISTS invoice_contingency (
    uf              CHAR(2)      NOT NULL,
    model           SMALLINT     NOT NULL,
    environment     VARCHAR(16)  NOT NULL,
    active          BOOLEAN      NOT NULL DEFAULT FALSE,
    kind            VARCHAR(6)   NOT NULL DEFAULT '', -- SVCAN | SVCRS
    reason          TEXT         NOT NULL DEFAULT '',
    started_at      TIMESTAMPTZ,
    -- Falhas de conexão seguidas com a autorizadora normal: três em dez minutos ativam a SVC.
    failures        INTEGER      NOT NULL DEFAULT 0,
    last_failure_at TIMESTAMPTZ,
    PRIMARY KEY (uf, model, environment)
);
