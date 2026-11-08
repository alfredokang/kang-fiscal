// Package domain contém as regras da numeração das notas.
package domain

import "errors"

var (
	ErrInvalidSeries  = errors.New("série inválida: de 0 a 999 (890 a 899 são do fisco)")
	ErrInvalidModel   = errors.New("modelo inválido: 55 (NF-e), 65 (NFC-e) ou 99 (DPS da NFS-e)")
	ErrInvalidNumber  = errors.New("número inválido: de 1 a 999.999.999")
	ErrNumberBackward = errors.New("o próximo número não pode ser menor ou igual a um número já reservado")
	ErrExhausted      = errors.New("a série chegou ao fim: use outra série")
	ErrNotFound       = errors.New("reserva de número não encontrada")
	ErrIssuerNotFound = errors.New("emitente não encontrado")
	ErrRequiredField  = errors.New("campo obrigatório")
)
