package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/pkg/adapter/queue"
)

type Period struct {
	ID            uuid.UUID `json:"id" db:"id"`
	Month         string    `json:"month" db:"month"`
	Gross         float64   `json:"gross" db:"gross"`
	InfraCost     float64   `json:"infra_cost" db:"infra_cost"`
	MaintainerFee float64   `json:"maintainer_fee" db:"maintainer_fee"`
	Net           float64   `json:"net" db:"net"`
	Status        string    `json:"status" db:"status"`
}

type ListPeriods struct{ db *sqlx.DB }

func NewListPeriods(db *sqlx.DB) *ListPeriods { return &ListPeriods{db: db} }

func (u *ListPeriods) Execute(ctx context.Context) ([]Period, error) {
	var list []Period
	err := sqlx.SelectContext(ctx, u.db, &list, `SELECT id, month, gross, infra_cost, maintainer_fee, net, status FROM pool_periods ORDER BY month DESC`)
	if err != nil {
		return nil, apperrors.FromDBError("pool.List", err)
	}
	return list, nil
}

type GetPeriod struct{ db *sqlx.DB }

func NewGetPeriod(db *sqlx.DB) *GetPeriod { return &GetPeriod{db: db} }

func (u *GetPeriod) Execute(ctx context.Context, month string) (Period, error) {
	var p Period
	err := sqlx.GetContext(ctx, u.db, &p, `SELECT id, month, gross, infra_cost, maintainer_fee, net, status FROM pool_periods WHERE month = $1`, month)
	if err != nil {
		return Period{}, apperrors.NotFound("pool.Get", err).WithDetail("período não encontrado")
	}
	return p, nil
}

type Distribute struct{ q *queue.Queue }

func NewDistribute(q *queue.Queue) *Distribute { return &Distribute{q: q} }

func (u *Distribute) Execute(ctx context.Context, month string) error {
	if u.q == nil {
		return apperrors.Internal("pool.Distribute", errNoQueue)
	}
	return u.q.EnqueueDistribute(ctx, month)
}

type errString string

func (e errString) Error() string { return string(e) }

var errNoQueue = errString("queue indisponível")

type Payout struct {
	ID         uuid.UUID `json:"id" db:"id"`
	CycleID    uuid.UUID `json:"cycle_id" db:"cycle_id"`
	AmountUSDC float64   `json:"amount_usdc" db:"amount_usdc"`
	Tx         string    `json:"tx" db:"tx"`
}

type ListPayouts struct{ db *sqlx.DB }

func NewListPayouts(db *sqlx.DB) *ListPayouts { return &ListPayouts{db: db} }

func (u *ListPayouts) Execute(ctx context.Context, walletID uuid.UUID) ([]Payout, error) {
	var list []Payout
	err := sqlx.SelectContext(ctx, u.db, &list, `
		SELECT id, cycle_id, amount_usdc, tx FROM payouts WHERE wallet_id = $1 ORDER BY created_at DESC`, walletID)
	if err != nil {
		return nil, apperrors.FromDBError("pool.Payouts", err)
	}
	return list, nil
}
