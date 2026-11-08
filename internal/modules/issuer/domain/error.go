// Package domain contém as regras do emitente — a empresa em nome de quem a nota sai.
package domain

import "errors"

var (
	ErrInvalidIssuerID        = errors.New("id de emitente inválido")
	ErrInvalidCNPJ            = errors.New("CNPJ inválido")
	ErrInvalidUF              = errors.New("UF inválida")
	ErrInvalidCityCode        = errors.New("código IBGE do município inválido: são 7 dígitos")
	ErrCityOutsideUF          = errors.New("o município não pertence à UF informada")
	ErrInvalidZipCode         = errors.New("CEP inválido: são 8 dígitos")
	ErrInvalidTaxRegime       = errors.New("regime tributário inválido: use 1 (Simples), 2 (Simples com excesso de sublimite), 3 (regime normal) ou 4 (MEI)")
	ErrInvalidStateReg        = errors.New("inscrição estadual inválida: só dígitos, ou ISENTO")
	ErrInvalidEnvironment     = errors.New("ambiente inválido: use homologation ou production")
	ErrRequiredField          = errors.New("campo obrigatório")
	ErrIssuerNotFound         = errors.New("emitente não encontrado")
	ErrIssuerAlreadyExists    = errors.New("já existe um emitente com este CNPJ")
	ErrIncompleteRegistration = errors.New("o cadastro do emitente está incompleto para emitir em produção")
	ErrAlreadyInProduction    = errors.New("o emitente já emite em produção")
)
