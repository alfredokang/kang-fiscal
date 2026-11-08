-- Perfis tributários (a regra fiscal de cada operação da empresa) e as alíquotas da
-- reforma por vigência (Constituição, Art. 12).

CREATE TABLE IF NOT EXISTS taxrule_profiles (
    tenant_id  VARCHAR(64)  NOT NULL,
    code       VARCHAR(40)  NOT NULL,
    name       VARCHAR(120) NOT NULL,
    -- O perfil é um valor: lido e gravado inteiro, sem consulta por campo. JSONB e não
    -- trinta colunas porque cada NT acrescenta campos, e o perfil é congelado na nota.
    rules      JSONB        NOT NULL,
    updated_at TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (tenant_id, code)
);

-- Alíquotas de IBS e CBS. Não têm tenant: são lei, iguais para todos os emitentes da
-- instalação. Local vazio = alíquota geral; preenchido prevalece sobre a geral.
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS taxrule_rates (
    id         UUID         PRIMARY KEY,
    kind       VARCHAR(8)   NOT NULL CHECK (kind IN ('cbs', 'ibs_uf', 'ibs_mun')),
    uf         VARCHAR(2)   NOT NULL DEFAULT '',
    city_code  VARCHAR(7)   NOT NULL DEFAULT '',
    value      NUMERIC(7,4) NOT NULL CHECK (value >= 0 AND value <= 100),
    valid_from DATE         NOT NULL,
    valid_to   DATE,
    source     TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CHECK (valid_to IS NULL OR valid_to >= valid_from),
    -- Duas alíquotas vigentes ao mesmo tempo para o mesmo tributo e local fariam a nota
    -- depender da ordem de leitura. O banco recusa, mesmo com duas gravações simultâneas.
    CONSTRAINT taxrule_rates_no_overlap EXCLUDE USING gist (
        kind WITH =, uf WITH =, city_code WITH =,
        daterange(valid_from, valid_to, '[]') WITH &&
    )
);

-- 2026, o ano de teste da reforma: CBS 0,9% e IBS 0,1%, iguais no país inteiro, destacados
-- e compensáveis com PIS/COFINS. De 2027 em diante NADA é semeado: as alíquotas ainda
-- dependem de resolução do Senado, e uma emissão sem alíquota cadastrada falha com
-- mensagem clara em vez de usar número inventado.
INSERT INTO taxrule_rates (id, kind, value, valid_from, valid_to, source) VALUES
    ('01900000-0000-7000-8000-000000002601', 'cbs',     0.9000, '2026-01-01', '2026-12-31', 'LC 214/2025 — alíquota de teste da CBS em 2026'),
    ('01900000-0000-7000-8000-000000002602', 'ibs_uf',  0.1000, '2026-01-01', '2026-12-31', 'LC 214/2025 — alíquota de teste do IBS em 2026'),
    ('01900000-0000-7000-8000-000000002603', 'ibs_mun', 0.0000, '2026-01-01', '2026-12-31', 'LC 214/2025 — em 2026 o IBS de teste é só estadual')
ON CONFLICT (id) DO NOTHING;
