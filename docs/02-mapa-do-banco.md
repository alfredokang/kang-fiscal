# Mapa do banco

Cada tabela, o módulo dono e o que ela guarda. Atualize ao criar ou remover tabela. O teste
`TestTodaTabelaTemDono` confere que toda tabela criada numa migration está registrada
como de algum módulo.

| Tabela | Módulo | O que guarda |
|---|---|---|
| `credential_keys` | `credential` | As credenciais de API por tenant: prefixo público, HMAC-SHA256 do segredo com o pepper do ambiente, escopos, último uso e revogação. O token completo nunca é guardado. |
| `dfe_cursors` | `dfe` | A posição da distribuição de cada emitente (último NSU, NSU máximo) e quando a próxima consulta pode sair — a SEFAZ bloqueia por uma hora quem consulta sem documento novo. |
| `dfe_documents` | `dfe` | As notas e os eventos recebidos contra o CNPJ: resumo (resNFe), XML completo (procNFe), resumos de evento; índice de emitente, valor e data; a manifestação registrada. NSU único por emitente. |
| `nfse_documents` | `nfse` | As NFS-e do padrão nacional: Id da DPS e DPS assinada gravados ANTES do envio (é pelo Id que se consulta depois de um timeout), número da DPS reservado no `numbering` (modelo 99), chave de acesso de 50 posições, número da NFS-e, estado (signed, authorized, rejected, uncertain, not_sent), código de erro E… e o XML da NFS-e com SHA-256. Gatilho recusa alterar ou apagar a NFS-e autorizada. Uma por Idempotency-Key no tenant. |
| `issuer_issuers` | `issuer` | O emitente: CNPJ (inclusive alfanumérico), IE, CRT, endereço fiscal com código IBGE e ambiente (homologação/produção). Um CNPJ por tenant. |
| `certificate_certificates` | `certificate` | O A1 de cada emitente: dados públicos (titular, CNPJ, AC, validade, fingerprint) e o `.pfx` e a senha **cifrados**. Um ativo por emitente; o segredo do aposentado é apagado. |
| `taxrule_profiles` | `taxrule` | Os perfis tributários do tenant (a regra fiscal de cada operação: CFOP, CST/CSOSN, alíquotas de ICMS, PIS, COFINS, IPI e a classificação do IBS/CBS), em JSONB. |
| `taxrule_rates` | `taxrule` | As alíquotas de CBS, IBS UF e IBS municipal por vigência e local. Sem tenant: são lei. Restrição de exclusão impede duas vigentes no mesmo período. Só 2026 vem semeado. |
| `numbering_series` | `numbering` | O próximo número de cada emitente, modelo, série e ambiente. Lida com `FOR UPDATE` na reserva. |
| `numbering_reservations` | `numbering` | Cada número entregue a um documento (`reserved`, `used`, `voided`). As abertas e antigas são os buracos a inutilizar. Número único por série. |
| `invoice_invoices` | `invoice` | (estado `offline` e `superseded_by` para a NFC-e em contingência) As notas: chave de acesso e XML assinado gravados ANTES do envio, estado (signed, authorized, denied, rejected, uncertain, not_sent), cStat, protocolo e o nfeProc com SHA-256. Gatilho recusa alterar o XML autorizado, apagar nota autorizada, denegada ou cancelada, e qualquer mudança de estado que não seja autorizada → cancelada. Uma por Idempotency-Key no tenant. |
| `invoice_contingency` | `invoice` | O estado de contingência por UF, modelo e ambiente: SVC ativa (SVCAN/SVCRS), motivo, início (o dhCont das notas) e as falhas de conexão seguidas. Sem tenant: a SEFAZ é a mesma para todos. |
| `invoice_events` | `invoice` | Cancelamentos (110111) e cartas de correção (110110): evento assinado gravado antes do envio, cStat, protocolo e o procEventoNFe. Registrado é imutável (gatilho). Tipo+sequência únicos por nota, exceto os rejeitados. |
| `invoice_voids` | `invoice` | As inutilizações de faixa: pedido assinado gravado antes do envio, cStat, protocolo, procInutNFe e as reservas que a faixa fecha. Homologada é imutável (gatilho). |
| `schema_migrations` | — | O que o `kang-fiscal migrate` já aplicou, por módulo e arquivo. |
