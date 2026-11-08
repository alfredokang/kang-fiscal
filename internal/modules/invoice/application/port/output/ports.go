// Package output declara o que a emissão precisa dos outros módulos e do mundo de fora.
// Cada porta é pequena e usa tipos daqui (Constituição, Art. 1.2 e 1.3).
package output

import (
	"context"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/entity"
	"github.com/alfredokang/kang-fiscal/internal/modules/invoice/domain/service"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
	"github.com/alfredokang/kang-fiscal/pkg/tax"
	"github.com/alfredokang/kang-fiscal/shared/pagination"
)

// IssuerReader lê o emitente para congelá-lo na nota.
type IssuerReader interface {
	// IssuerForInvoice devolve o snapshot e o que falta no cadastro (vazio = pode emitir).
	IssuerForInvoice(ctx context.Context, tenantID, issuerRef string) (service.IssuerSnapshot, []string, error)
}

// SignerProvider entrega o certificado A1 aberto, só em memória.
type SignerProvider interface {
	SignerFor(ctx context.Context, tenantID, issuerRef string) (*certificate.Certificate, error)
}

// TaxProfiles lê o perfil tributário do item.
type TaxProfiles interface {
	TaxProfile(ctx context.Context, tenantID, code string) (tax.Profile, error)
}

// TaxRates resolve as alíquotas de IBS/CBS vigentes.
type TaxRates interface {
	TaxRatesAt(ctx context.Context, date time.Time, uf, cityCode string) (tax.Rates, error)
}

// NumberKey identifica a sequência da nota.
type NumberKey struct {
	TenantID, IssuerRef, Environment string
	Model, Series                    int
}

// NumberReserver reserva e fecha números.
type NumberReserver interface {
	ReserveNumber(ctx context.Context, key NumberKey, documentRef string) (reservationRef string, number int, err error)
	MarkNumberUsed(ctx context.Context, tenantID, reservationRef string) error
	MarkNumberVoided(ctx context.Context, tenantID, reservationRef string) error
	// PendingNumbers lista os números reservados há mais que o corte e ainda abertos.
	PendingNumbers(ctx context.Context, before time.Time, limit int) ([]PendingNumber, error)
}

// PendingNumber é um número reservado e ainda sem destino.
type PendingNumber struct {
	ReservationRef, DocumentRef string
	Key                         NumberKey
	Number                      int
}

// SefazGateway fala com a autorizadora usando o certificado do emitente.
type SefazGateway interface {
	Authorize(ctx context.Context, cert *certificate.Certificate, t sefaz.Target, batchID int64, signed []byte) (sefaz.AuthorizeResult, error)
	Query(ctx context.Context, cert *certificate.Certificate, t sefaz.Target, key string) (sefaz.QueryResult, error)
	SendEvent(ctx context.Context, cert *certificate.Certificate, t sefaz.Target, batchID int64, signed []byte) (sefaz.EventResult, error)
	VoidRange(ctx context.Context, cert *certificate.Certificate, t sefaz.Target, signed []byte) (sefaz.VoidResult, error)
}

// VoidRepository guarda as inutilizações.
type VoidRepository interface {
	CreateVoid(ctx context.Context, v *entity.Void) error
	UpdateVoid(ctx context.Context, v *entity.Void) error
	ListVoids(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.Void, int64, error)
}

// EventRepository guarda os eventos das notas.
type EventRepository interface {
	CreateEvent(ctx context.Context, e *entity.Event) error
	UpdateEvent(ctx context.Context, e *entity.Event) error
	// Events lista os eventos da nota, em ordem; no máximo 40 (1 cancelamento + 20 CC-e e
	// as tentativas rejeitadas).
	Events(ctx context.Context, tenantID, accessKey string) ([]*entity.Event, error)
	FindEvent(ctx context.Context, tenantID, id string) (*entity.Event, error)
}

// InvoiceRepository guarda as notas.
type InvoiceRepository interface {
	// Create grava a nota assinada ANTES do envio (Constituição, Art. 6.1). Uma
	// Idempotency-Key repetida no tenant devolve domain.ErrIdempotencyConflict.
	Create(ctx context.Context, inv *entity.Invoice) error
	Update(ctx context.Context, inv *entity.Invoice) error
	FindByIdempotencyKey(ctx context.Context, tenantID, key string) (*entity.Invoice, error)
	FindByAccessKey(ctx context.Context, tenantID, accessKey string) (*entity.Invoice, error)
	FindByID(ctx context.Context, tenantID, id string) (*entity.Invoice, error)
	// InRange lista as notas da série com número na faixa (no máximo a faixa inteira).
	InRange(ctx context.Context, key NumberKey, from, to int) ([]*entity.Invoice, error)
	List(ctx context.Context, tenantID string, page pagination.Params) ([]*entity.Invoice, int64, error)
}
