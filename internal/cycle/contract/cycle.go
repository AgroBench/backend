package contract

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/cycle/domain"
	"github.com/AgroBench/backend/internal/cycle/types/input"
	"github.com/AgroBench/backend/internal/cycle/types/output"
)

type CycleRepo interface {
	Create(ctx context.Context, c domain.Cycle) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Cycle, error)
	List(ctx context.Context, cultureID, regionID, status string) ([]domain.Cycle, error)
	Close(ctx context.Context, id uuid.UUID) error
	MarkAggregated(ctx context.Context, id uuid.UUID) error
}

type Create interface {
	Execute(ctx context.Context, in input.CreateCycle) (output.Cycle, error)
}

type List interface {
	Execute(ctx context.Context, in input.ListCycles) ([]output.Cycle, error)
}

type Close interface {
	Execute(ctx context.Context, id uuid.UUID) (output.Cycle, error)
}
