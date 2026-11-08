-- O certificado A1 de cada emitente.
--
-- O .pfx e a senha só existem aqui CIFRADOS (AES-256-GCM, chave fora do banco —
-- Constituição, Art. 9.1). Quando o certificado é trocado ou revogado, o segredo do antigo
-- é APAGADO (NULL) e só os dados públicos ficam, para o histórico de quem assinou o quê.
CREATE TABLE IF NOT EXISTS certificate_certificates (
    id              UUID         PRIMARY KEY,
    tenant_id       VARCHAR(64)  NOT NULL,
    -- O emitente é de outro módulo: texto opaco, sem FK (Art. 2).
    issuer_ref      UUID         NOT NULL,
    holder_name     VARCHAR(200) NOT NULL,
    cnpj            CHAR(14)     NOT NULL,
    ca_name         VARCHAR(200) NOT NULL,
    serial          VARCHAR(80)  NOT NULL,
    fingerprint     CHAR(64)     NOT NULL,
    not_before      TIMESTAMPTZ  NOT NULL,
    not_after       TIMESTAMPTZ  NOT NULL,
    sealed_pfx      TEXT,
    sealed_password TEXT,
    active          BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ  NOT NULL,
    retired_at      TIMESTAMPTZ,
    -- Ativo sem segredo não assina; inativo com segredo é segredo esquecido.
    CONSTRAINT certificate_certificates_secret_only_when_active
        CHECK ((active AND sealed_pfx IS NOT NULL AND sealed_password IS NOT NULL)
            OR (NOT active AND sealed_pfx IS NULL AND sealed_password IS NULL))
);

-- Um ativo por emitente: dois ativos e cada nota assinaria com um diferente.
CREATE UNIQUE INDEX IF NOT EXISTS certificate_certificates_one_active_idx
    ON certificate_certificates (tenant_id, issuer_ref) WHERE active;

-- O aviso de vencimento varre os ativos por data.
CREATE INDEX IF NOT EXISTS certificate_certificates_expiry_idx
    ON certificate_certificates (not_after) WHERE active;
