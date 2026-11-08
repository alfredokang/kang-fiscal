-- O emitente: a empresa em nome de quem a nota sai.
--
-- `tenant_id` é texto opaco, sem FK (Constituição, Art. 2): o tenant é de outro contexto, e
-- FK entre módulos impede que este vire serviço.
CREATE TABLE IF NOT EXISTS issuer_issuers (
    id                     UUID         PRIMARY KEY,
    tenant_id              VARCHAR(64)  NOT NULL,
    -- 14 posições: o CNPJ alfanumérico (julho de 2026) tem letras nas 12 primeiras.
    cnpj                   CHAR(14)     NOT NULL,
    legal_name             VARCHAR(60)  NOT NULL,
    trade_name             VARCHAR(60)  NOT NULL DEFAULT '',
    -- Texto, porque ISENTO é valor válido.
    state_registration     VARCHAR(14)  NOT NULL DEFAULT '',
    municipal_registration VARCHAR(15)  NOT NULL DEFAULT '',
    cnae                   VARCHAR(7)   NOT NULL DEFAULT '',
    -- O CRT do XML: 1 Simples, 2 Simples com excesso, 3 regime normal, 4 MEI.
    tax_regime             SMALLINT     NOT NULL CHECK (tax_regime BETWEEN 1 AND 4),
    street                 VARCHAR(60)  NOT NULL DEFAULT '',
    number                 VARCHAR(60)  NOT NULL DEFAULT '',
    complement             VARCHAR(60)  NOT NULL DEFAULT '',
    district               VARCHAR(60)  NOT NULL DEFAULT '',
    city_code              CHAR(7),
    city_name              VARCHAR(60)  NOT NULL DEFAULT '',
    uf                     CHAR(2)      NOT NULL,
    zip_code               CHAR(8),
    phone                  VARCHAR(14)  NOT NULL DEFAULT '',
    -- Nasce em homologação (Constituição, Art. 6.4). Só o caso de uso de promoção muda.
    environment            VARCHAR(16)  NOT NULL DEFAULT 'homologation'
                           CHECK (environment IN ('homologation', 'production')),
    created_at             TIMESTAMPTZ  NOT NULL,
    updated_at             TIMESTAMPTZ  NOT NULL,
    -- O mesmo CNPJ em dois emitentes do mesmo tenant dividiria a numeração em duas
    -- sequências — e a SEFAZ recusa a segunda nota com o mesmo número.
    CONSTRAINT issuer_issuers_tenant_cnpj_key UNIQUE (tenant_id, cnpj)
);

CREATE INDEX IF NOT EXISTS issuer_issuers_tenant_created_idx
    ON issuer_issuers (tenant_id, created_at DESC);
