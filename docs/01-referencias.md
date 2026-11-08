# Referências

> Levantamento de 2026-11-08. As estrelas e a data do último commit são daquele dia, e
> envelhecem.

## Regra de reaproveitamento

| Licença | Pode copiar código? |
|---|---|
| MIT, BSD, Apache-2.0 | **Sim**, mantendo o aviso de copyright original no arquivo e o crédito no `NOTICE` |
| LGPL, GPL, MPL | **Não.** Só se estuda o desenho; o código é escrito do zero |
| Sem licença | **Não.** Sem licença, todos os direitos são reservados ao autor |

## Em Go

| Projeto | Licença | Situação | Uso aqui |
|---|---|---|---|
| [mschunke/gonfe](https://github.com/mschunke/gonfe) | MIT | NF-e, NFC-e, CT-e, MDF-e, eventos, inutilização, DistribuiçãoDF-e e DANFE. Sem CGO, sem `float64`, assinatura sem reserializar. Um autor só, e **sem IBS/CBS** (o próprio `HANDOFF.md` lista a reforma como prioridade crítica que falta) | **Referência principal** de desenho para `pkg/nfe`, `xmldsig` e `sefaz`. Usa identificadores em português, então não é dependência direta |
| [frones/nfe](https://pkg.go.dev/github.com/frones/nfe) | MPL-2.0 | Parado desde 2023 | Só consulta |
| [fabyo/go-nfe-validator](https://github.com/fabyo/go-nfe-validator) | MIT | Valida o XML contra o XSD 4.00 | Ideia para a etapa opcional de XSD no CI |
| [engmarcosdeogenes/nfe-go](https://github.com/engmarcosdeogenes/nfe-go) | sem licença | — | Não usar |

## Reforma tributária, em outras linguagens

Contei as ocorrências de `IBSCBS` no código de cada projeto.

| Projeto | Licença | `IBSCBS` | Uso aqui |
|---|---|---|---|
| [Unimake/DFe](https://github.com/Unimake/DFe) (C#) | MIT | 236 | A cobertura mais completa da reforma. Referência campo a campo do grupo UB; também cobre NFS-e |
| [nfephp-org/sped-nfe](https://github.com/nfephp-org/sped-nfe) (PHP) | LGPL/GPL **ou MIT** | 36 | O emissor mais usado do Brasil. Fixtures de teste e conferência cruzada de XML. O [sped-da](https://github.com/nfephp-org/sped-da) é a referência de DANFE |
| [Samuel-Oliveira/Java_NFe](https://github.com/Samuel-Oliveira/Java_NFe) | MIT | 34 | Segunda conferência dos grupos da reforma |
| [ZeusAutomacao/DFe.NET](https://github.com/ZeusAutomacao/DFe.NET) (C#) | LGPL-2.1 | 67 | Só estudo |
| [TadaSoftware/PyNFe](https://github.com/TadaSoftware/PyNFe) | LGPL | 7 | Só estudo |

## NFS-e Nacional

| Projeto | Licença | Uso aqui |
|---|---|---|
| [Gomes343/PyNFSeNacional](https://github.com/Gomes343/PyNFSeNacional) | MIT | DPS, mTLS, DANFSe local e cancelamento; feito por um contador, já emitiu em homologação |
| [gildonei/nfse-nacional](https://github.com/gildonei/nfse-nacional) (PHP) | MIT | Segunda referência |

## Dependências

| Biblioteca | Licença | Para quê |
|---|---|---|
| [SSLMate/go-pkcs12](https://github.com/SSLMate/go-pkcs12) | BSD-3 | Ler o `.pfx` A1 |
| [skip2/go-qrcode](https://github.com/skip2/go-qrcode) | MIT | Codificar o QR Code do DANFCE (correção de erro testada por milhares de projetos) |
| [makiuchi-d/gozxing](https://github.com/makiuchi-d/gozxing) | Apache-2.0 | Só nos testes: ler de volta o QR Code, de forma independente do codificador |
| [russellhaering/goxmldsig](https://github.com/russellhaering/goxmldsig) | Apache-2.0 | Alternativa de XMLDSig. A preferência é assinar sem reserializar, como o gonfe faz |

## Tabelas e cadeia de confiança

| Fonte | Licença | Uso aqui |
|---|---|---|
| sped-nfe `storage/wsnfe_4.00_mod55.xml`, `mod65.xml`, `autorizadores.json`, `cstat.json` | MIT | `pkg/sefaz/endpoints.json` e `cstat.json`, gerados por `scripts/sefaz-tables.py` |
| [rffontenelle/CertificadosAC-ICP-Brasil](https://github.com/rffontenelle/CertificadosAC-ICP-Brasil) | CC0 | Os intermediários da ICP-Brasil em `pkg/sefaz/icpbrasil.pem`, aceitos só se a assinatura fechar numa raiz fixada (`scripts/icpbundle`) |
| Repositório de raízes da Microsoft (CCADB), via [sslyze](https://github.com/nabla-c0d3/sslyze) | — | Conferência independente do SHA-256 das raízes v5 e v10 |

**Achado de 2026-11-08:** a tabela de endereços do gonfe trata o RS como parte da SVRS e
aponta a SVRS para o host da SEFAZ-RS. O RS tem autorizador próprio. Vale abrir uma issue
lá.

**Achados do teste real (2026-11-08, com certificado de teste):**
- A SEFAZ-SP recusa HTTP/2 (`HTTP_1_1_REQUIRED`): o cliente força HTTP/1.1.
- 11 de 13 autorizadoras usam certificado TLS ICP-Brasil, que nenhum sistema traz; GO e
  SVAN usam AC pública.
- MG, PR, AM e MT recusam o certificado no handshake; SP, RS, BA, PE, MA e AC respondem
  HTTP 403; GO responde SOAP com cStat 999.

## Normas

- **NT 2025.002 (RTC)**: grupos de IBS, CBS e IS na NF-e/NFC-e. Na v1.40, nota sem IBS/CBS é
  rejeitada desde 03/08/2026 no regime normal; Simples e MEI a partir de 2027.
  ([Systax](https://www.systax.com.br/?p=42952), [notagateway](https://notagateway.com.br/?p=8409))
- **IN RFB nº 2.229/2024**: CNPJ alfanumérico, atribuído a partir de julho de 2026.
  Implementado em `pkg/document`.
- **LC 214/2025**: regulamenta IBS, CBS e IS.
- **Calculadora de tributos da RFB**: tem versão offline em contêiner
  (`api-regime-geral.jar`). ([Machado Meyer](https://www.machadomeyer.com.br/pt/inteligencia-juridica/publicacoes-ij/tributario-ij/rfb-libera-ferramenta-de-calculo-da-reforma-tributaria-para-toda-a-sociedade))

**Antes de implementar cada grupo do leiaute, confira a versão vigente da NT no Portal da
NF-e.** As versões mudam várias vezes por ano, e este levantamento é de uma data só.
