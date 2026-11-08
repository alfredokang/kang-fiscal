-- Os eventos da nota: cancelamento (110111) e carta de correção (110110). O evento
-- registrado é documento fiscal, imutável como a nota (Constituição, Art. 6.3).
CREATE TABLE IF NOT EXISTS invoice_events (
    id          UUID         PRIMARY KEY,
    tenant_id   VARCHAR(64)  NOT NULL,
    invoice_id  UUID         NOT NULL REFERENCES invoice_invoices (id), -- mesma fronteira: FK permitida
    access_key  VARCHAR(44)  NOT NULL,
    type        CHAR(6)      NOT NULL CHECK (type IN ('110111', '110110')),
    seq         SMALLINT     NOT NULL CHECK (seq BETWEEN 1 AND 20),
    status      VARCHAR(12)  NOT NULL CHECK (status IN ('signed', 'registered', 'rejected', 'uncertain', 'not_sent')),
    cstat       INTEGER      NOT NULL DEFAULT 0,
    motive      TEXT         NOT NULL DEFAULT '',
    protocol    VARCHAR(20)  NOT NULL DEFAULT '',
    text        TEXT         NOT NULL, -- justificativa ou correção
    signed_xml  BYTEA        NOT NULL,
    proc_xml    BYTEA,
    created_at  TIMESTAMPTZ  NOT NULL,
    updated_at  TIMESTAMPTZ  NOT NULL
);

-- O mesmo tipo e sequência numa nota é o mesmo evento para a SEFAZ. O rejeitado fica de
-- fora: a SEFAZ não o registrou, e a próxima tentativa usa a mesma sequência.
CREATE UNIQUE INDEX IF NOT EXISTS invoice_events_unique
    ON invoice_events (access_key, type, seq) WHERE status <> 'rejected';

CREATE INDEX IF NOT EXISTS invoice_events_invoice_idx ON invoice_events (invoice_id, created_at);

CREATE OR REPLACE FUNCTION invoice_protect_event() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'registered' THEN
        RAISE EXCEPTION 'o evento % da nota % está registrado e não muda', OLD.type, OLD.access_key;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS invoice_protect_event ON invoice_events;
CREATE TRIGGER invoice_protect_event BEFORE UPDATE OR DELETE ON invoice_events
    FOR EACH ROW EXECUTE FUNCTION invoice_protect_event();
