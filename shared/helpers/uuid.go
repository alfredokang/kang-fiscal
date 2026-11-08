package helpers

import "github.com/google/uuid"

// GenerateUUIDv7 gera um UUID v7 — ordenável por tempo.
//
// v7 e não v4 por dois motivos: o índice da primary key cresce sequencialmente em vez de
// fragmentar a cada insert, e o ID existe antes do INSERT, que é o que torna possível
// gravar entidade e evento de domínio na mesma transação (Constituição, Art. 4.2).
func GenerateUUIDv7() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// IsValidUUID valida o formato de um identificador recebido de fora.
func IsValidUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
