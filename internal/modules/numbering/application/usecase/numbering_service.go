// Package usecase orquestra a numeração.
package usecase

import (
	"context"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain/entity"
)

// NumberingService é a fachada do módulo.
type NumberingService struct {
	repo    output.SeriesRepository
	issuers output.IssuerChecker
	now     func() time.Time
}

// NewNumberingService monta o serviço.
func NewNumberingService(repo output.SeriesRepository, issuers output.IssuerChecker) *NumberingService {
	return &NumberingService{repo: repo, issuers: issuers, now: func() time.Time { return time.Now().UTC() }}
}

// Now é o relógio do serviço (os adapters convertem corte absoluto em idade com ele).
func (s *NumberingService) Now() time.Time { return s.now() }

// Reserve entrega o próximo número para um documento.
func (s *NumberingService) Reserve(ctx context.Context, key entity.SeriesKey, documentRef string) (entity.Reservation, error) {
	if err := key.Validate(); err != nil {
		return entity.Reservation{}, err
	}
	if documentRef == "" {
		return entity.Reservation{}, domain.ErrRequiredField
	}
	return s.repo.Reserve(ctx, key, documentRef, s.now())
}

// SetNext ajusta o próximo número de uma série.
func (s *NumberingService) SetNext(ctx context.Context, key entity.SeriesKey, next int) error {
	if err := key.Validate(); err != nil {
		return err
	}
	if next < 1 || next > entity.MaxNumber {
		return domain.ErrInvalidNumber
	}
	ok, err := s.issuers.IssuerExists(ctx, key.TenantID, key.IssuerRef)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrIssuerNotFound
	}
	return s.repo.SetNext(ctx, key, next)
}

// MarkUsed fecha a reserva: o número chegou à SEFAZ como nota autorizada ou denegada.
func (s *NumberingService) MarkUsed(ctx context.Context, tenantID, reservationID string) error {
	return s.repo.Settle(ctx, tenantID, reservationID, entity.StatusUsed, s.now())
}

// MarkVoided fecha a reserva depois da inutilização homologada.
func (s *NumberingService) MarkVoided(ctx context.Context, tenantID, reservationID string) error {
	return s.repo.Settle(ctx, tenantID, reservationID, entity.StatusVoided, s.now())
}

// PendingVoid lista os números reservados há mais de `age` sem nota: os buracos a
// inutilizar. A inutilização tem de ser pedida até o dia 10 do mês seguinte.
func (s *NumberingService) PendingVoid(ctx context.Context, age time.Duration, limit int) ([]entity.Reservation, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	return s.repo.PendingVoid(ctx, s.now().Add(-age), limit)
}
