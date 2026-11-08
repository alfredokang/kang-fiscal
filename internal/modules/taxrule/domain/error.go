// Package domain contém as regras fiscais da empresa e as alíquotas da reforma por vigência.
package domain

import "errors"

var (
	ErrInvalidCode     = errors.New("código do perfil inválido: use de 2 a 40 letras minúsculas, números ou hífen")
	ErrRequiredField   = errors.New("campo obrigatório")
	ErrInvalidProfile  = errors.New("perfil tributário inválido")
	ErrProfileNotFound = errors.New("perfil tributário não encontrado")
	ErrInvalidRate     = errors.New("alíquota inválida: de 0 a 100, com até 4 casas")
	ErrInvalidPeriod   = errors.New("vigência inválida: o fim não pode vir antes do início")
	ErrRateOverlap     = errors.New("já existe alíquota vigente para este tributo, local e período")
	// ErrNoRate: não há alíquota vigente para a data. A emissão para aqui em vez de usar
	// um número inventado — as alíquotas definitivas da reforma ainda não foram fixadas
	// (Constituição, Art. 12).
	ErrNoRate = errors.New("não há alíquota de IBS/CBS cadastrada para esta data e local")
)
