-- As inutilizações de faixas de numeração. A homologada é documento fiscal: não muda e não
-- some (Constituição, Art. 6.3).
CREATE TABLE IF NOT EXISTS invoice_voids (
    id               UUID         PRIMARY KEY,
    tenant_id        VARCHAR(64)  NOT NULL,
    issuer_ref       UUID         NOT NULL,
    environment      VARCHAR(16)  NOT NULL,
    model            SMALLINT     NOT NULL,
    series           SMALLINT     NOT NULL,
    number_from      INTEGER      NOT NULL,
    number_to        INTEGER      NOT NULL CHECK (number_to >= number_from),
    justification    TEXT         NOT NULL,
    -- As reservas do numbering que esta faixa fecha, como texto opaco (Art. 2), separadas
    -- por vírgula (são UUIDs).
    reservation_refs TEXT         NOT NULL DEFAULT '',
    status           VARCHAR(12)  NOT NULL CHECK (status IN ('signed', 'voided', 'rejected', 'uncertain', 'not_sent')),
    cstat            INTEGER      NOT NULL DEFAULT 0,
    motive           TEXT         NOT NULL DEFAULT '',
    protocol         VARCHAR(20)  NOT NULL DEFAULT '',
    signed_xml       BYTEA        NOT NULL,
    proc_xml         BYTEA,
    created_at       TIMESTAMPTZ  NOT NULL,
    updated_at       TIMESTAMPTZ  NOT NULL
);

CREATE INDEX IF NOT EXISTS invoice_voids_tenant_idx ON invoice_voids (tenant_id, created_at DESC);

CREATE OR REPLACE FUNCTION invoice_protect_void() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'voided' THEN
        RAISE EXCEPTION 'a inutilização %-% está homologada e não muda', OLD.number_from, OLD.number_to;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS invoice_protect_void ON invoice_voids;
CREATE TRIGGER invoice_protect_void BEFORE UPDATE OR DELETE ON invoice_voids
    FOR EACH ROW EXECUTE FUNCTION invoice_protect_void();
