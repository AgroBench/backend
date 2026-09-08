package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/cycle/domain"
)

type row struct {
	ID            uuid.UUID `db:"id"`
	CultureID     uuid.UUID `db:"culture_id"`
	MicroRegionID uuid.UUID `db:"micro_region_id"`
	Label         string    `db:"label"`
	OpensAt       time.Time `db:"opens_at"`
	ClosesAt      time.Time `db:"closes_at"`
	Status        string    `db:"status"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

func (r row) toDomain() domain.Cycle {
	return domain.Cycle{
		ID: r.ID, CultureID: r.CultureID, MicroRegionID: r.MicroRegionID,
		Label: r.Label, OpensAt: r.OpensAt, ClosesAt: r.ClosesAt,
		Status: domain.CycleStatus(r.Status), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type CycleRepo struct{ db sqlx.ExtContext }

func New(db sqlx.ExtContext) *CycleRepo { return &CycleRepo{db: db} }

func (r *CycleRepo) Create(ctx context.Context, c domain.Cycle) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cycles (id, culture_id, micro_region_id, label, opens_at, closes_at, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		c.ID, c.CultureID, c.MicroRegionID, c.Label, c.OpensAt, c.ClosesAt, c.Status)
	if err != nil {
		return apperrors.FromDBError("cycle.Create", err)
	}
	return nil
}

func (r *CycleRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Cycle, error) {
	var rec row
	err := sqlx.GetContext(ctx, r.db, &rec, `
		SELECT id, culture_id, micro_region_id, label, opens_at, closes_at, status, created_at, updated_at
		  FROM cycles WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Cycle{}, apperrors.NotFound("cycle.GetByID", err).WithDetail("ciclo não encontrado")
	}
	if err != nil {
		return domain.Cycle{}, apperrors.FromDBError("cycle.GetByID", err)
	}
	return rec.toDomain(), nil
}

func (r *CycleRepo) List(ctx context.Context, cultureID, regionID, status string) ([]domain.Cycle, error) {
	q := `SELECT id, culture_id, micro_region_id, label, opens_at, closes_at, status, created_at, updated_at FROM cycles WHERE 1=1`
	args := []any{}
	i := 1
	if cultureID != "" {
		q += ` AND culture_id = $` + strconv.Itoa(i)
		args = append(args, cultureID)
		i++
	}
	if regionID != "" {
		q += ` AND micro_region_id = $` + strconv.Itoa(i)
		args = append(args, regionID)
		i++
	}
	if status != "" {
		q += ` AND status = $` + strconv.Itoa(i)
		args = append(args, status)
	}
	q += ` ORDER BY opens_at DESC`
	var rows []row
	if err := sqlx.SelectContext(ctx, r.db, &rows, q, args...); err != nil {
		return nil, apperrors.FromDBError("cycle.List", err)
	}
	out := make([]domain.Cycle, len(rows))
	for i, rec := range rows {
		out[i] = rec.toDomain()
	}
	return out, nil
}

func (r *CycleRepo) Close(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `UPDATE cycles SET status = 'closed' WHERE id = $1 AND status = 'open'`, id)
	if err != nil {
		return apperrors.FromDBError("cycle.Close", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.Conflict("cycle.Close", errors.New("not open")).WithDetail("ciclo não está aberto")
	}
	return nil
}

func (r *CycleRepo) MarkAggregated(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE cycles SET status = 'aggregated' WHERE id = $1`, id)
	if err != nil {
		return apperrors.FromDBError("cycle.MarkAggregated", err)
	}
	return nil
}
