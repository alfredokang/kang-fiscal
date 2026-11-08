// Package domain contém as regras do certificado A1 do emitente.
package domain

import "errors"

var (
	ErrInvalidID          = errors.New("id inválido")
	ErrEmptyFile          = errors.New("envie o arquivo .pfx do certificado")
	ErrFileTooLarge       = errors.New("o arquivo é grande demais para um certificado A1")
	ErrWrongPassword      = errors.New("senha incorreta ou arquivo de certificado corrompido")
	ErrNotRSA             = errors.New("o certificado não usa chave RSA, que a NF-e exige")
	ErrExpired            = errors.New("o certificado está fora do período de validade")
	ErrNoCNPJ             = errors.New("o certificado não é de pessoa jurídica: não traz CNPJ")
	ErrCNPJMismatch       = errors.New("o certificado é de outra empresa: a raiz do CNPJ não confere com a do emitente")
	ErrNotFound           = errors.New("o emitente não tem certificado cadastrado")
	ErrIssuerNotFound     = errors.New("emitente não encontrado")
	ErrEncryptionDisabled = errors.New("o servidor está sem SECRETS_ENCRYPTION_KEY: o certificado não pode ser guardado")
)
