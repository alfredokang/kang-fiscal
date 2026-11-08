-- A reserva idempotente por venda. claim_key é a Idempotency-Key da emissão: duas
-- requisições da MESMA venda chegando juntas recebem a MESMA reserva (mesmo número e mesmo
-- documento), em vez de gastar dois números — o segundo virava buraco a inutilizar.
ALTER TABLE numbering_reservations ADD COLUMN IF NOT EXISTS claim_key VARCHAR(80);

CREATE UNIQUE INDEX IF NOT EXISTS numbering_reservations_claim
    ON numbering_reservations (tenant_id, issuer_ref, model, series, environment, claim_key)
    WHERE claim_key IS NOT NULL;
