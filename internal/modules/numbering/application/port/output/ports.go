// Package output declara o que a numeração precisa do mundo de fora.
package output

import (
	"context"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain/entity"
)

// SeriesRepository reserva números sob lock.
type SeriesRepository interface {
	// Reserve entrega o próximo número da série numa transação com SELECT … FOR UPDATE:
	// duas emissões ao mesmo tempo nunca recebem o mesmo número. A série nasce em 1.
	Reserve(ctx context.Context, key entity.SeriesKey, documentRef string, at time.Time) (entity.Reservation, error)
	// SetNext ajusta o próximo número (migração de outro sistema). Recusa voltar para
	// número já reservado.
	SetNext(ctx context.Context, key entity.SeriesKey, next int) error
	// Settle marca a reserva como usada ou inutilizada.
	Settle(ctx context.Context, tenantID, reservationID, status string, at time.Time) error
	// PendingVoid lista as reservas ainda abertas, mais antigas que o corte: são os buracos
	// que precisam de inutilização.
	PendingVoid(ctx context.Context, before time.Time, limit int) ([]entity.Reservation, error)
}

// IssuerChecker confirma que o emitente existe no tenant. O emitente é de outro módulo.
type IssuerChecker interface {
	IssuerExists(ctx context.Context, tenantID, issuerRef string) (bool, error)
}
