-- NFC-e em contingência offline (tpEmis 9): a nota emitida sem a SEFAZ, pendente de
-- transmissão, e o vínculo com a nota online que ela substituiu.
ALTER TABLE invoice_invoices DROP CONSTRAINT IF EXISTS invoice_invoices_status_check;
ALTER TABLE invoice_invoices ADD CONSTRAINT invoice_invoices_status_check CHECK (status IN
    ('signed', 'authorized', 'denied', 'rejected', 'uncertain', 'not_sent', 'canceled', 'offline'));

-- superseded_by: a nota online que ficou sem resposta aponta para a NFC-e offline que
-- cobriu a mesma venda. Ela nunca é reenviada: se tiver sido autorizada, é cancelada por
-- substituição (110112); se a SEFAZ não a conhece, o número vai para a inutilização.
ALTER TABLE invoice_invoices ADD COLUMN IF NOT EXISTS superseded_by UUID;

CREATE INDEX IF NOT EXISTS invoice_invoices_offline_idx ON invoice_invoices (created_at)
    WHERE status = 'offline' OR superseded_by IS NOT NULL;

ALTER TABLE invoice_events DROP CONSTRAINT IF EXISTS invoice_events_type_check;
ALTER TABLE invoice_events ADD CONSTRAINT invoice_events_type_check CHECK (type IN ('110111', '110110', '110112'));
