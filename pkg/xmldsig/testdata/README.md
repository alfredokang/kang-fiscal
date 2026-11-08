# Documentos reais assinados

Vêm dos fixtures do [sped-nfe](https://github.com/nfephp-org/sped-nfe) (distribuído também
sob licença MIT), com a indentação removida: nos originais ela foi acrescentada **depois** da
assinatura, e quebrava o resumo. Nenhum outro byte foi alterado.

Eles são a prova de que a canonicalização daqui produz os mesmos bytes que o assinador de
quem emitiu e que a SEFAZ conferiu. Se um teste com estes arquivos falhar, o erro está no
`c14n.go`, nunca no arquivo.

| Arquivo | Origem no sped-nfe |
|---|---|
| `nfe_310_sp_assinada.xml` | `tests/fixtures/xml/2017signed.xml` |
| `nfe_400_rs_autorizada.xml` | `tests/fixtures/xml/nfe_layout4_com_prot.xml` (com protocolo) |
| `nfce_400_ba_assinada.xml` | `tests/fixtures/xml/nfce_com_qrcode.xml` |
| `evento_cce_sp.xml` | `tests/fixtures/xml/exemplo_xml_request_cce_cnpj.xml` |
