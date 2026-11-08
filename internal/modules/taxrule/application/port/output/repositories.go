// Package output declara o que o módulo taxrule precisa do mundo de fora.
package output

import (
	"context"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/taxrule/domain/entity"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// ProfileRepository guarda os perfis do tenant.
type ProfileRepository interface {
	Save(ctx context.Context, p *entity.Profile) error
	Find(ctx context.Context, tenantID, code string) (*entity.Profile, error)
	List(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.Profile, int64, error)
}

// RateRepository guarda as alíquotas por vigência.
type RateRepository interface {
	// Add devolve domain.ErrRateOverlap se o período cruza outro do mesmo tributo e local.
	Add(ctx context.Context, r *entity.Rate) error
	// Candidates devolve as alíquotas vigentes na data que podem valer para o local.
	Candidates(ctx context.Context, date time.Time, uf, city string) ([]*entity.Rate, error)
	List(ctx context.Context, page pagination.Params) ([]*entity.Rate, int64, error)
}
