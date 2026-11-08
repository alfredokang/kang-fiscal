# Mapa do banco

Cada tabela, o módulo dono e o que ela guarda. Atualize ao criar ou remover tabela. O teste
`TestTodaTabelaTemDono` confere que toda tabela criada numa migration está registrada
como de algum módulo.

| Tabela | Módulo | O que guarda |
|---|---|---|
| `issuer_issuers` | `issuer` | O emitente: CNPJ (inclusive alfanumérico), IE, CRT, endereço fiscal com código IBGE e ambiente (homologação/produção). Um CNPJ por tenant. |
| `certificate_certificates` | `certificate` | O A1 de cada emitente: dados públicos (titular, CNPJ, AC, validade, fingerprint) e o `.pfx` e a senha **cifrados**. Um ativo por emitente; o segredo do aposentado é apagado. |
| `schema_migrations` | — | O que o `kang-fiscal migrate` já aplicou, por módulo e arquivo. |
