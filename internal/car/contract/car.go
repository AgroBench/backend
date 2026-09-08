package contract

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/car/domain"
	"github.com/AgroBench/backend/internal/car/types/input"
	"github.com/AgroBench/backend/internal/car/types/output"
)

type PropertyRepo interface {
	Create(ctx context.Context, p domain.Property) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Property, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Property, error)
}

type CatalogRepo interface {
	ListMicroRegions(ctx context.Context) ([]domain.MicroRegion, error)
	GetMicroRegion(ctx context.Context, id uuid.UUID) (domain.MicroRegion, error)
	ListCultures(ctx context.Context) ([]domain.Culture, error)
	GetCulture(ctx context.Context, id uuid.UUID) (domain.Culture, error)
}

type CreateProperty interface {
	Execute(ctx context.Context, userID uuid.UUID, in input.CreateProperty) (output.Property, error)
}

type GetProperty interface {
	Execute(ctx context.Context, userID uuid.UUID) (output.Property, error)
}
