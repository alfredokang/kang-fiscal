-- Modelo 99: a sequência da DPS (NFS-e Nacional). A DPS tem numeração própria do
-- prestador e não tem inutilização: número sem NFS-e não precisa ser comunicado.
ALTER TABLE numbering_series DROP CONSTRAINT IF EXISTS numbering_series_model_check;
ALTER TABLE numbering_series ADD CONSTRAINT numbering_series_model_check CHECK (model IN (55, 65, 99));
