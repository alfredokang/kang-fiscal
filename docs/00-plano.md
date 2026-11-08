# Plano do Kang Fiscal

> Decidido em 2026-11-08. Este documento diz **o que vem em cada fase** e por quê. As
> regras de como o código é escrito estão na [Constituição](../CONSTITUTION.md).

## A ideia

Quem precisa emitir nota no Brasil hoje tem duas saídas: pagar um provedor por nota, ou
usar uma biblioteca em PHP, C#, Java ou Delphi. Em Go não existe um emissor completo, e
nenhum dos que existem cobre a reforma tributária (ver [referências](01-referencias.md)).

O Kang Fiscal entrega duas coisas no mesmo repositório:

1. **A API em Docker**, para qualquer linguagem: `docker run alfredokang/kang-fiscal`.
2. **A biblioteca Go (`pkg/`)**, para quem quer embutir a emissão sem subir outro serviço.

A API usa a própria biblioteca por dentro: o código fiscal é um só.

## Por que a reforma tributária está no MVP, e não numa fase posterior

Pela NT 2025.002 (v1.40), a NF-e e a NFC-e do **regime normal sem o grupo IBS/CBS são
rejeitadas desde 03/08/2026**. Simples Nacional e MEI entram em 2027. Um emissor sem os
grupos da reforma já nasce inútil para o regime normal.

O que **não** se sabe ainda, e por isso é dado e não código (Constituição, Art. 12):

- As alíquotas definitivas de CBS e IBS, que dependem de resolução do Senado. Em 2026
  valem as de teste: CBS 0,9% e IBS 0,1%.
- O calendário fino da transição: a CBS e o IS entram em 2027, e o IBS sobe gradualmente
  até 2033.
- As próximas versões das notas técnicas.

**Split payment.** Quem faz o split é o PSP, na liquidação, não o emissor, e a
regulamentação operacional ainda não saiu. O emissor deixa pronto o que vai ser
necessário: por nota, a chave de acesso e os valores de IBS, CBS e IS, num endpoint e num
webhook (`invoice.split_data`).

## Módulos

| Módulo | Responsabilidade | Fase |
|---|---|---|
| `issuer` | Emitente: CNPJ, IE, CRT, endereço fiscal, ambiente | ✅ F0 |
| `credential` | Credenciais `kf_` por tenant (HMAC com pepper, escopos setup/nfe:write/nfe:read, revogação, rate limit em memória), mostradas uma única vez; tabela de escopos que nega o que não lista | ✅ F1 |
| `certificate` | A1 `.pfx` cifrado em repouso, raiz do CNPJ conferida, segredo do antigo apagado na troca, nunca devolvido | ✅ F1 |
| `numbering` | (inutilização automática pelo worker do `invoice`: rejeitada, não enviada e reserva sem nota; incerta é consultada antes e nunca inutilizada) Série e número com `FOR UPDATE` (50 reservas simultâneas sem buraco nem repetição), homologação e produção separadas, ajuste para quem migra sem poder voltar, lista de buracos a inutilizar | ✅ F1 |
| `taxrule` | Perfis tributários do tenant, alíquotas de IBS/CBS por vigência e local (só 2026 semeado; sem alíquota, a emissão para), `POST /v1/admin/tax/simulate` | ✅ F1 |
| `invoice` | `POST /v1/nfe` (NF-e 55): Idempotency-Key, número, tributos pelo perfil, XML assinado e chave gravados antes do envio, consulta pela chave quando incerto, duplicidade tratada como consulta, nfeProc imutável com hash. Cancelamento e carta de correção com procEventoNFe, reenvio seguro (573 = registrado). Faltam: NFC-e e DANFE | ✅ F1 |
| `contingency` (no `invoice`) | SVC-AN/SVC-RS da NF-e: ligada por 108/109 (reemite a mesma venda com o mesmo número) ou por 3 falhas de conexão em 10 min (só as próximas notas; a incerta nunca vai para a SVC); sonda no worker desliga quando a UF volta; manual em /v1/admin/contingency. NFC-e offline (tpEmis 9): a venda não espera a SEFAZ — sai na hora com novo número e QR Code v3 assinado (RSA-SHA1, conferido contra o sped-nfe); a online sem resposta fica substituída, nunca reenviada; o worker transmite a offline e, se a online tinha sido autorizada, cancela por substituição (110112), senão libera o número para a inutilização. Falta: EPEC | ✅ F2 |
| `dfe` | Distribuição de DF-e por NSU (resumos, XML completo e eventos contra o CNPJ), espera de 1 hora depois de 137/656 aplicada no domínio e no worker, manifestação do destinatário (ciência, confirmação, desconhecimento, não realizada) validada no XSD, download do XML completo pela chave depois da ciência | ✅ F4 |
| `nfse` | NFS-e no padrão nacional (DPS + Sefin Nacional) | F5 |
| `webhook` | Eventos de saída assinados (HMAC) com outbox e retentativa | F2 |
| `audit` | Trilha de quem fez o quê, com segredos redigidos | F2 |

Biblioteca (`pkg/`):

| Pacote | O que faz | Fase |
|---|---|---|
| `decimal` | Ponto fixo, escala do leiaute, arredondamento explícito | ✅ F0 |
| `document` | CNPJ (inclusive alfanumérico) e CPF | ✅ F0 |
| `accesskey` | Chave de 44 posições, cNF aleatório | ✅ F0 |
| `nfe` | Leiaute 4.00 (PL_010 v1.30): tributos atuais (ICMS 00/20/40/60/90, Simples 101/102/500/900, IPI, PIS, COFINS, DIFAL) **e** IBS/CBS/IS lado a lado; totais somados item a item; validado contra o XSD oficial | ✅ F1 |
| `xmldsig` | Assinatura enveloped C14N 1.0 própria, inserida sem reserializar; conferida contra NF-e reais | ✅ F1 |
| `certificate` | Leitura do A1 (`go-pkcs12`), CNPJ alfanumérico no otherName, nunca impresso em log | ✅ F1 |
| `sefaz` | Cliente SOAP 1.2 com mTLS e cadeia ICP-Brasil verificada; status, autorização síncrona, consulta pela chave; 528 códigos de retorno com dica; URLs das 27 UFs em tabela | ✅ F1 |
| `tax` | Cálculo de ICMS, FCP, IPI, PIS, COFINS e IBS/CBS (com redução de alíquota), arredondando uma vez por valor | ✅ F1 |
| `danfe` | DANFCE (bobina 80 mm) com QR Code (`skip2/go-qrcode`, MIT), lido de volta da imagem do PDF por decodificador independente; DANFE em PDF (gerador mínimo adaptado do gonfe, MIT): canhoto, código de barras Code 128C conferido por decodificação, quebra de página, IBS/CBS nos dados adicionais, carimbo de homologação e de cancelada; imprime nota de qualquer emissor | ✅ F1 |

## API (v1)

```
POST   /v1/nfe                      emite (síncrono com teto → 202 + webhook se demorar)
GET    /v1/nfe/{chave}              consulta (local, ou ?sync=sefaz)
GET    /v1/nfe/{chave}/xml | /danfe
POST   /v1/nfe/{chave}/cancel       cancelamento
POST   /v1/nfe/{chave}/correction   carta de correção
POST   /v1/nfe/void-range           inutilização
GET    /v1/nfe?status=&from=&to=    lista paginada por cursor
POST   /v1/nfce …                   o mesmo para o modelo 65
POST   /v1/tax/simulate             calcula os tributos SEM emitir (útil no carrinho)
GET    /v1/dfe/received             notas contra o CNPJ + manifestação
POST   /v1/nfse …
GET    /v1/sefaz/status/{uf}
/v1/admin/issuers …                 ✅ cadastro do emitente
```

`POST /v1/tax/simulate` é o diferencial para e-commerce: a loja mostra os tributos no
carrinho com o mesmo motor que vai emitir a nota.

## Fases

| Fase | Entrega | Pronto quando |
|---|---|---|
| **F0 — Fundação** ✅ | Repositório, constituição, guards, Postgres 18, imagem distroless, `pkg/decimal`, `pkg/document`, `pkg/accesskey`, módulo `issuer` | `make check` verde e imagem rodando |
| **F1 — NF-e 55 em homologação** | `credential`, `certificate`, `numbering`, `taxrule`, `invoice`; `pkg/nfe`, `xmldsig`, `sefaz`, `danfe`; grupos IBS/CBS/IS | Nota autorizada em homologação em SP, MG e na SVRS |
| **F2 — NFC-e 65** | QR Code + CSC, DANFCE, webhooks, auditoria | Venda de balcão emitida e impressa |
| **F3 — Contingência** | SVC automática, EPEC, offline, consulta antes de reenviar sob caos | SEFAZ caindo no meio do envio não gera nota duplicada |
| **F4 — Recebimento** | DistribuiçãoDF-e, manifestação | — |
| **F5 — NFS-e Nacional** ✅ | DPS + Sefin Nacional, com IBS/CBS; consulta da DPS pelo Id antes de reenviar; cancelamento direto (e101101) e por análise fiscal (e101103), conferido na lista de eventos; DANFSe v2.0 (NT 008/2026) gerado localmente | DPS validada no XSD da NT 009 e aceita em homologação |
| **F6 — Ecossistema** | SDKs gerados do OpenAPI (TS, PHP, Python), site de documentação; depois CT-e e MDF-e | — |

**Primeiro cliente:** o backend da Minas Café. O módulo `invoice` previsto lá vira uma
porta `InvoiceIssuerPort` com um adapter que chama esta API. É a prova de que o emissor se
incorpora a qualquer projeto.

## Conformidade

- **Golden XML** (Constituição, Art. 11.3): cada cenário fiscal gera um XML comparado byte a
  byte com um arquivo conferido na SEFAZ de homologação.
- **Conferência cruzada com o sped-nfe** (MIT): a mesma entrada, nos dois emissores, deve
  produzir o mesmo XML antes da assinatura.
- **Calculadora oficial da RFB** como verificação opcional: ela tem versão offline em
  contêiner. Entra como serviço opcional no `docker-compose` (`?verify=rfb`), **sem**
  redistribuir a imagem, porque a licença dela não está clara. O usuário baixa da Receita.

## Riscos

- **Homologação exige certificado A1 real.** Sem ele, a F1 se valida só por golden XML.
- **Cada UF tem suas peculiaridades.** As URLs e as particularidades ficam em tabela, e a
  suíte de conformidade tem um caso por UF.
- **Não existe validador XSD maduro em Go puro**, e o caminho com cgo e libxml2 quebra a
  imagem leve. Por isso: structs tipadas, regras da NT no domínio, e XSD como etapa
  opcional de CI. A SEFAZ continua sendo a validação final.
