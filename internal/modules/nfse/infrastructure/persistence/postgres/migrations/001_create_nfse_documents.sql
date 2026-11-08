-- As NFS-e (padrão nacional). A DPS assinada e o Id dela são gravados ANTES do envio
-- (Constituição, Art. 6.1): depois de um timeout é pelo Id que se consulta o Sistema
-- Nacional. A NFS-e autorizada não muda nem some.
CREATE TABLE IF NOT EXISTS nfse_documents (
    id              UUID         PRIMARY KEY,
    tenant_id       VARCHAR(64)  NOT NULL,
    issuer_ref      UUID         NOT NULL, -- texto opaco ao issuer (Art. 2)
    idempotency_key VARCHAR(80)  NOT NULL,
    request_hash    CHAR(64)     NOT NULL,
    series          INTEGER      NOT NULL,
    number          INTEGER      NOT NULL,
    reservation_ref UUID         NOT NULL,
    environment     VARCHAR(16)  NOT NULL,
    dps_id          CHAR(45)     NOT NULL,
    access_key      VARCHAR(50),
    nfse_number     VARCHAR(15)  NOT NULL DEFAULT '',
    status          VARCHAR(12)  NOT NULL CHECK (status IN ('signed', 'authorized', 'rejected', 'uncertain', 'not_sent')),
    error_code      VARCHAR(10)  NOT NULL DEFAULT '',
    message         TEXT         NOT NULL DEFAULT '',
    total           NUMERIC(15,2) NOT NULL,
    signed_xml      BYTEA        NOT NULL,
    nfse_xml        BYTEA,
    xml_hash        CHAR(64),
    created_at      TIMESTAMPTZ  NOT NULL,
    updated_at      TIMESTAMPTZ  NOT NULL,
    CONSTRAINT nfse_documents_idempotency UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT nfse_documents_dps UNIQUE (dps_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS nfse_documents_key ON nfse_documents (access_key) WHERE access_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS nfse_documents_tenant_idx ON nfse_documents (tenant_id, created_at DESC);

CREATE OR REPLACE FUNCTION nfse_protect_authorized() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'authorized' THEN
        RAISE EXCEPTION 'a NFS-e % está autorizada e não muda', OLD.access_key;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS nfse_protect_authorized ON nfse_documents;
CREATE TRIGGER nfse_protect_authorized BEFORE UPDATE OR DELETE ON nfse_documents
    FOR EACH ROW EXECUTE FUNCTION nfse_protect_authorized();
