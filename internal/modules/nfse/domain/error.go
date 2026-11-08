// Package domain contém as regras da emissão da NFS-e Nacional.
package domain

import "errors"

var (
	ErrIdempotencyKey      = errors.New("envie o cabeçalho Idempotency-Key (até 80 caracteres): é ele que impede a mesma nota de sair duas vezes")
	ErrIdempotencyConflict = errors.New("esta Idempotency-Key já foi usada com outro conteúdo")
	ErrInvalidRequest      = errors.New("pedido de emissão de NFS-e inválido")
	ErrIssuerNotFound      = errors.New("emitente não encontrado")
	ErrIssuerIncomplete    = errors.New("o cadastro do emitente está incompleto para a NFS-e")
	ErrNoCertificate       = errors.New("o emitente não tem certificado A1 válido cadastrado")
	ErrNotFound            = errors.New("NFS-e não encontrada")
	ErrNotSent             = errors.New("a DPS não chegou a ser processada pelo Sistema Nacional")
	ErrNotAuthorized       = errors.New("só uma NFS-e autorizada pode ser cancelada")
	ErrInvalidEvent        = errors.New("pedido de cancelamento de NFS-e inválido")
)
