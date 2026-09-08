package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/car/domain"
)

type propRow struct {
	ID            uuid.UUID  `db:"id"`
	UserID        uuid.UUID  `db:"user_id"`
	CARHMAC       string     `db:"car_hmac"`
	MicroRegionID uuid.UUID  `db:"micro_region_id"`
	CARStatus     string     `db:"car_status"`
	VerifiedAt    *time.Time `db:"verified_at"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

func (r propRow) toDomain() domain.Property {
	return domain.Property{
		ID: r.ID, UserID: r.UserID, CARHMAC: r.CARHMAC,
		MicroRegionID: r.MicroRegionID, CARStatus: domain.CARStatus(r.CARStatus),
		VerifiedAt: r.VerifiedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type PropertyRepo struct{ db sqlx.ExtContext }

func NewProperty(db sqlx.ExtContext) *PropertyRepo { return &PropertyRepo{db: db} }

func (r *PropertyRepo) Create(ctx context.Context, p domain.Property) error {
	const op = "car.Create"
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO properties (id, user_id, car_hmac, micro_region_id, car_status, verified_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.UserID, p.CARHMAC, p.MicroRegionID, p.CARStatus, p.VerifiedAt)
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	return nil
}

func (r *PropertyRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Property, error) {
	return r.get(ctx, "car.GetByUserID", `SELECT id, user_id, car_hmac, micro_region_id, car_status, verified_at, created_at, updated_at FROM properties WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID)
}

func (r *PropertyRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Property, error) {
	return r.get(ctx, "car.GetByID", `SELECT id, user_id, car_hmac, micro_region_id, car_status, verified_at, created_at, updated_at FROM properties WHERE id = $1`, id)
}

func (r *PropertyRepo) get(ctx context.Context, op, q string, arg any) (domain.Property, error) {
	var rec propRow
	err := sqlx.GetContext(ctx, r.db, &rec, q, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Property{}, apperrors.NotFound(op, err).WithDetail("propriedade não encontrada")
	}
	if err != nil {
		return domain.Property{}, apperrors.FromDBError(op, err)
	}
	return rec.toDomain(), nil
}

type CatalogRepo struct{ db sqlx.ExtContext }

func NewCatalog(db sqlx.ExtContext) *CatalogRepo { return &CatalogRepo{db: db} }

func (r *CatalogRepo) ListMicroRegions(ctx context.Context) ([]domain.MicroRegion, error) {
	var list []domain.MicroRegion
	err := sqlx.SelectContext(ctx, r.db, &list, `SELECT id, ibge_code, name, uf FROM micro_regions ORDER BY uf, name`)
	if err != nil {
		return nil, apperrors.FromDBError("car.ListMicroRegions", err)
	}
	return list, nil
}

func (r *CatalogRepo) GetMicroRegion(ctx context.Context, id uuid.UUID) (domain.MicroRegion, error) {
	var m domain.MicroRegion
	err := sqlx.GetContext(ctx, r.db, &m, `SELECT id, ibge_code, name, uf FROM micro_regions WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MicroRegion{}, apperrors.NotFound("car.GetMicroRegion", err).WithDetail("microrregião não encontrada")
	}
	if err != nil {
		return domain.MicroRegion{}, apperrors.FromDBError("car.GetMicroRegion", err)
	}
	return m, nil
}

func (r *CatalogRepo) ListCultures(ctx context.Context) ([]domain.Culture, error) {
	var list []domain.Culture
	err := sqlx.SelectContext(ctx, r.db, &list, `SELECT id, code, name FROM cultures ORDER BY name`)
	if err != nil {
		return nil, apperrors.FromDBError("car.ListCultures", err)
	}
	return list, nil
}

func (r *CatalogRepo) GetCulture(ctx context.Context, id uuid.UUID) (domain.Culture, error) {
	var c domain.Culture
	err := sqlx.GetContext(ctx, r.db, &c, `SELECT id, code, name FROM cultures WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Culture{}, apperrors.NotFound("car.GetCulture", err).WithDetail("cultura não encontrada")
	}
	if err != nil {
		return domain.Culture{}, apperrors.FromDBError("car.GetCulture", err)
	}
	return c, nil
}
