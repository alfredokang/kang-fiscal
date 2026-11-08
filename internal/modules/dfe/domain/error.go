// Package domain contém as regras do recebimento de notas (distribuição de DF-e).
package domain

import "errors"

var (
	ErrNotFound       = errors.New("documento não encontrado")
	ErrIssuerNotFound = errors.New("emitente não encontrado")
	ErrNoCertificate  = errors.New("o emitente não tem certificado A1 válido: a distribuição exige o certificado do destinatário")
	ErrInvalidRequest = errors.New("pedido inválido")
	ErrTooSoon        = errors.New("a SEFAZ só permite nova consulta depois do horário indicado (evita o bloqueio por consumo indevido)")
	ErrNotSummary     = errors.New("só o resumo de uma NF-e (resNFe) pode ser manifestado")
)
