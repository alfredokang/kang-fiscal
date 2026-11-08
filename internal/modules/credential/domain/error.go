// Package domain contém as regras das credenciais de API.
package domain

import "errors"

var (
	ErrInvalidTenant = errors.New("tenant inválido: de 2 a 64 letras minúsculas, números ou hífen")
	ErrInvalidScope  = errors.New("escopo inválido: use setup, nfe:write ou nfe:read")
	ErrRequiredField = errors.New("campo obrigatório")
	ErrNoPepper      = errors.New("o servidor está sem CREDENTIAL_PEPPER: credenciais não podem ser criadas nem conferidas")
	ErrUnauthorized  = errors.New("credencial inválida")
	ErrNotFound      = errors.New("credencial não encontrada")
)
