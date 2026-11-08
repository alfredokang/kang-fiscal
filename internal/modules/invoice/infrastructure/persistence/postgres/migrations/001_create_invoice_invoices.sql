-- As notas emitidas. A linha nasce ANTES do envio, com a chave de acesso e o XML assinado
-- (Constituição, Art. 6.1); o XML autorizado (nfeProc) é imutável depois de gravado (6.3).
CREATE TABLE IF NOT EXISTS invoice_invoices (
    id              UUID         PRIMARY KEY,
    tenant_id       VARCHAR(64)  NOT NULL,
    issuer_ref      UUID         NOT NULL, -- texto opaco ao issuer, sem FK (Art. 2)
    idempotency_key VARCHAR(80)  NOT NULL,
    request_hash    CHAR(64)     NOT NULL,
    model           SMALLINT     NOT NULL,
    series          SMALLINT     NOT NULL,
    number          INTEGER      NOT NULL,
    reservation_ref UUID         NOT NULL, -- a reserva do numbering, texto opaco
    environment     VARCHAR(16)  NOT NULL,
    access_key      VARCHAR(44)  NOT NULL,
    status          VARCHAR(12)  NOT NULL CHECK (status IN
                        ('signed', 'authorized', 'denied', 'rejected', 'uncertain', 'not_sent')),
    cstat           INTEGER      NOT NULL DEFAULT 0,
    motive          TEXT         NOT NULL DEFAULT '',
    protocol        VARCHAR(20)  NOT NULL DEFAULT '',
    total_amount    NUMERIC(15,2) NOT NULL,
    signed_xml      BYTEA        NOT NULL,
    authorized_xml  BYTEA,
    xml_hash        CHAR(64),
    created_at      TIMESTAMPTZ  NOT NULL,
    updated_at      TIMESTAMPTZ  NOT NULL,
    authorized_at   TIMESTAMPTZ,
    CONSTRAINT invoice_invoices_idempotency UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT invoice_invoices_access_key UNIQUE (access_key)
);

CREATE INDEX IF NOT EXISTS invoice_invoices_tenant_created_idx ON invoice_invoices (tenant_id, created_at DESC);
-- A conciliação varre as notas que ficaram sem resposta conclusiva.
CREATE INDEX IF NOT EXISTS invoice_invoices_pending_idx ON invoice_invoices (updated_at)
    WHERE status IN ('signed', 'uncertain');

-- Nota autorizada não muda nem some: o XML autorizado, o protocolo e a chave são o
-- documento fiscal, e a guarda é de 5 anos. O gatilho vale para qualquer cliente do banco,
-- não só para este código.
CREATE OR REPLACE FUNCTION invoice_protect_authorized() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.status IN ('authorized', 'denied') THEN
            RAISE EXCEPTION 'nota % (%) não pode ser apagada', OLD.access_key, OLD.status;
        END IF;
        RETURN OLD;
    END IF;
    IF OLD.authorized_xml IS NOT NULL AND (
        NEW.authorized_xml IS DISTINCT FROM OLD.authorized_xml OR NEW.xml_hash IS DISTINCT FROM OLD.xml_hash OR
        NEW.access_key <> OLD.access_key OR NEW.protocol <> OLD.protocol OR NEW.signed_xml <> OLD.signed_xml) THEN
        RAISE EXCEPTION 'o XML autorizado da nota % é imutável', OLD.access_key;
    END IF;
    IF OLD.status IN ('authorized', 'denied') AND NEW.status <> OLD.status THEN
        RAISE EXCEPTION 'nota % já está %: o estado só muda por evento', OLD.access_key, OLD.status;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS invoice_protect_authorized ON invoice_invoices;
CREATE TRIGGER invoice_protect_authorized BEFORE UPDATE OR DELETE ON invoice_invoices
    FOR EACH ROW EXECUTE FUNCTION invoice_protect_authorized();
