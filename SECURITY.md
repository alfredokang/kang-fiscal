# Segurança

O Kang Fiscal guarda certificados digitais e emite documentos com valor legal. Uma falha
aqui pode expor o certificado de uma empresa ou permitir emitir nota em nome dela.

## Como relatar uma falha

**Não abra issue pública.** Uma falha descrita em público fica exposta para todos que usam
o emissor antes da correção.

Use o relato privado do GitHub: aba **Security → Report a vulnerability** neste
repositório. Descreva o que encontrou, como reproduzir e o impacto que você imagina.

A resposta inicial vem em até 7 dias. Depois de corrigido, o problema é publicado com
crédito a quem relatou, se a pessoa quiser.

## Versões com correção

Só a versão mais recente da imagem `alfredokang/kang-fiscal` recebe correções de
segurança. Fixe uma versão em produção, mas acompanhe as novas.

## O que o projeto já faz

- O certificado A1 e a senha dele ficam cifrados no banco (AES-256-GCM), com a chave só no
  ambiente (`SECRETS_ENCRYPTION_KEY`).
- Os segredos das credenciais de API são guardados como HMAC-SHA256; o token aparece uma
  vez só.
- A conexão com a SEFAZ e com a Sefin confere a cadeia do servidor (ICP-Brasil e raízes
  públicas), nunca `InsecureSkipVerify`.
- A imagem é distroless, sem shell, e roda como usuário sem privilégio.
