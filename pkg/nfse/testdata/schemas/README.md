# Schemas da DPS (NFS-e Nacional)

`20261020/`: o pacote `esquemas_prod_restrita_20261020.zip` da NT 009, publicado pela
SE/CGNFS-e em gov.br/nfse e em vigor na produção restrita (homologação) desde 31/10/2026.
Copiado de github.com/Gomes343/PyNFSeNacional (MIT), que o usa nos próprios testes.

Toda DPS que o `pkg/nfse` monta é validada contra ele (`xmllint --schema`).

`20261020/pedRegEvento_v1.01.xsd` e `tiposEventos_v1.01.xsd`: o pedido de registro de evento
(cancelamento e101101, análise fiscal e101103), que o pacote do PyNFSeNacional não traz.
Copiados de github.com/akretion/nfelib (MIT), `nfelib/nfse/schemas/v1_0`. Eles incluem o
`tiposComplexos_v1.01.xsd` deste mesmo diretório.
