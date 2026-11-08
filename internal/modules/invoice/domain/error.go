// Package domain contém as regras da emissão da nota.
package domain

import "errors"

var (
	ErrIdempotencyKey      = errors.New("envie o cabeçalho Idempotency-Key (até 80 caracteres): é ele que impede a mesma nota de sair duas vezes")
	ErrIdempotencyConflict = errors.New("esta Idempotency-Key já foi usada com outro conteúdo")
	ErrRequiredField       = errors.New("campo obrigatório")
	ErrInvalidRequest      = errors.New("pedido de emissão inválido")
	ErrNotSupported        = errors.New("operação ainda não suportada pelo Kang Fiscal")
	ErrIssuerIncomplete    = errors.New("o cadastro do emitente está incompleto")
	ErrNotFound            = errors.New("nota não encontrada")
	ErrIssuerNotFound      = errors.New("emitente não encontrado")
	ErrNoCertificate       = errors.New("o emitente não tem certificado A1 válido cadastrado")
	ErrTaxProfileNotFound  = errors.New("perfil tributário não encontrado")
	ErrTaxRateMissing      = errors.New("não há alíquota de IBS/CBS cadastrada para a data da emissão")
	ErrNotAuthorized       = errors.New("só nota autorizada aceita cancelamento ou carta de correção")
	ErrAlreadyCanceled     = errors.New("a nota já está cancelada")
	ErrTooManyCorrections  = errors.New("a nota já tem 20 cartas de correção, o máximo permitido")
	// ErrNotSent: a SEFAZ recusou a conexão (certificado) ou o servidor não é confiável.
	// Nada foi processado; a nota pode ser emitida de novo com o mesmo pedido.
	ErrNotSent = errors.New("a nota não chegou a ser processada pela SEFAZ")
)
