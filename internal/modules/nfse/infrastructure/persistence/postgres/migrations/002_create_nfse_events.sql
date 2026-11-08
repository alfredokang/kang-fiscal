-- Os eventos da NFS-e: cancelamento (e101101) e pedido de análise fiscal (e101103). Uma
-- linha por nota e tipo: a tentativa recusada é refeita na mesma linha; o evento registrado
-- não muda nem some. O pedido assinado é gravado ANTES do envio (Constituição, Art. 6.1).
CREATE TABLE IF NOT EXISTS nfse_events (
    id            UUID         PRIMARY KEY,
    tenant_id     VARCHAR(64)  NOT NULL,
    document_id   UUID         NOT NULL REFERENCES nfse_documents (id),
    kind          CHAR(6)      NOT NULL CHECK (kind IN ('101101', '101103')),
    reason        SMALLINT     NOT NULL CHECK (reason IN (1, 2, 9)),
    justification TEXT         NOT NULL,
    status        VARCHAR(12)  NOT NULL CHECK (status IN ('signed', 'registered', 'rejected', 'uncertain', 'not_sent')),
    error_code    VARCHAR(10)  NOT NULL DEFAULT '',
    message       TEXT         NOT NULL DEFAULT '',
    signed_xml    BYTEA        NOT NULL,
    event_xml     BYTEA,
    created_at    TIMESTAMPTZ  NOT NULL,
    updated_at    TIMESTAMPTZ  NOT NULL,
    CONSTRAINT nfse_events_one_per_kind UNIQUE (document_id, kind)
);

CREATE INDEX IF NOT EXISTS nfse_events_pending_idx ON nfse_events (updated_at) WHERE status = 'uncertain';

CREATE OR REPLACE FUNCTION nfse_protect_event() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'registered' THEN
        RAISE EXCEPTION 'o evento % da NFS-e já foi registrado e não muda', OLD.kind;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS nfse_protect_event ON nfse_events;
CREATE TRIGGER nfse_protect_event BEFORE UPDATE OR DELETE ON nfse_events
    FOR EACH ROW EXECUTE FUNCTION nfse_protect_event();
