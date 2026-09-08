package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/cycle/contract"
	"github.com/AgroBench/backend/internal/cycle/domain"
	"github.com/AgroBench/backend/internal/cycle/types/input"
	"github.com/AgroBench/backend/internal/cycle/types/output"
	"github.com/AgroBench/backend/pkg/adapter/queue"
)

type Create struct{ repo contract.CycleRepo }

func NewCreate(repo contract.CycleRepo) *Create { return &Create{repo: repo} }

func (u *Create) Execute(ctx context.Context, in input.CreateCycle) (output.Cycle, error) {
	const op = "cycle.Create"
	if !in.ClosesAt.After(in.OpensAt) {
		return output.Cycle{}, apperrors.Validation(op, errors.New("dates")).WithDetail("closes_at deve ser depois de opens_at")
	}
	c := domain.Cycle{
		ID: coredomain.NewID(), CultureID: in.CultureID, MicroRegionID: in.MicroRegionID,
		Label: in.Label, OpensAt: in.OpensAt, ClosesAt: in.ClosesAt, Status: domain.CycleOpen,
	}
	if err := u.repo.Create(ctx, c); err != nil {
		if apperrors.Is(err, apperrors.ErrConflict) {
			return output.Cycle{}, apperrors.Conflict(op, err).WithDetail("ciclo já existe para cultura × região × safra")
		}
		return output.Cycle{}, err
	}
	return output.NewCycle(c), nil
}

type List struct{ repo contract.CycleRepo }

func NewList(repo contract.CycleRepo) *List { return &List{repo: repo} }

func (u *List) Execute(ctx context.Context, in input.ListCycles) ([]output.Cycle, error) {
	list, err := u.repo.List(ctx, in.CultureID, in.RegionID, in.Status)
	if err != nil {
		return nil, err
	}
	return output.NewCycles(list), nil
}

type Close struct {
	repo  contract.CycleRepo
	queue *queue.Queue
}

func NewClose(repo contract.CycleRepo, q *queue.Queue) *Close { return &Close{repo: repo, queue: q} }

func (u *Close) Execute(ctx context.Context, id uuid.UUID) (output.Cycle, error) {
	if err := u.repo.Close(ctx, id); err != nil {
		return output.Cycle{}, err
	}
	if u.queue != nil {
		if err := u.queue.EnqueueAggregate(ctx, id); err != nil {
			return output.Cycle{}, apperrors.Internal("cycle.Close", err)
		}
	}
	c, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return output.Cycle{}, err
	}
	return output.NewCycle(c), nil
}
