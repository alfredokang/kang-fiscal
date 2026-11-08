// Package output declara o que as credenciais precisam do mundo de fora.
package output

import (
	"context"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain/entity"
)

// KeyRepository guarda as credenciais.
type KeyRepository interface {
	Create(ctx context.Context, k *entity.Key) error
	FindByPrefix(ctx context.Context, prefix string) (*entity.Key, error)
	// List lista as credenciais do tenant (no máximo 200 — uma por sistema integrado).
	List(ctx context.Context, tenantID string) ([]*entity.Key, error)
	Revoke(ctx context.Context, id string, at time.Time) error
	// Touch registra o uso, no máximo uma escrita por minuto por chave.
	Touch(ctx context.Context, id string, at time.Time) error
}
