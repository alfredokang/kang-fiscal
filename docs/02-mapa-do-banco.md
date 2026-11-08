# Mapa do banco

Cada tabela, o módulo dono e o que ela guarda. Atualize ao criar ou remover tabela. O teste
`TestTodaTabelaTemDono` confere que toda tabela criada numa migration está registrada
como de algum módulo.

| Tabela | Módulo | O que guarda |
|---|---|---|
| `issuer_issuers` | `issuer` | O emitente: CNPJ (inclusive alfanumérico), IE, CRT, endereço fiscal com código IBGE e ambiente (homologação/produção). Um CNPJ por tenant. |
| `certificate_certificates` | `certificate` | O A1 de cada emitente: dados públicos (titular, CNPJ, AC, validade, fingerprint) e o `.pfx` e a senha **cifrados**. Um ativo por emitente; o segredo do aposentado é apagado. |
| `taxrule_profiles` | `taxrule` | Os perfis tributários do tenant (a regra fiscal de cada operação: CFOP, CST/CSOSN, alíquotas de ICMS, PIS, COFINS, IPI e a classificação do IBS/CBS), em JSONB. |
| `taxrule_rates` | `taxrule` | As alíquotas de CBS, IBS UF e IBS municipal por vigência e local. Sem tenant: são lei. Restrição de exclusão impede duas vigentes no mesmo período. Só 2026 vem semeado. |
| `numbering_series` | `numbering` | O próximo número de cada emitente, modelo, série e ambiente. Lida com `FOR UPDATE` na reserva. |
| `numbering_reservations` | `numbering` | Cada número entregue a um documento (`reserved`, `used`, `voided`). As abertas e antigas são os buracos a inutilizar. Número único por série. |
| `invoice_invoices` | `invoice` | As notas: chave de acesso e XML assinado gravados ANTES do envio, estado (signed, authorized, denied, rejected, uncertain, not_sent), cStat, protocolo e o nfeProc com SHA-256. Gatilho recusa alterar o XML autorizado, apagar nota autorizada, denegada ou cancelada, e qualquer mudança de estado que não seja autorizada → cancelada. Uma por Idempotency-Key no tenant. |
| `invoice_events` | `invoice` | Cancelamentos (110111) e cartas de correção (110110): evento assinado gravado antes do envio, cStat, protocolo e o procEventoNFe. Registrado é imutável (gatilho). Tipo+sequência únicos por nota, exceto os rejeitados. |
| `schema_migrations` | — | O que o `kang-fiscal migrate` já aplicou, por módulo e arquivo. |
