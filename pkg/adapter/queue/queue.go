package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/spf13/viper"
)

type ValidateContributionArgs struct {
	ContributionID uuid.UUID `json:"contribution_id"`
}

func (ValidateContributionArgs) Kind() string { return "validate_contribution" }

func (ValidateContributionArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type AggregateCycleArgs struct {
	CycleID uuid.UUID `json:"cycle_id"`
}

func (AggregateCycleArgs) Kind() string { return "aggregate_cycle" }

func (AggregateCycleArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type DistributePoolArgs struct {
	Month string `json:"month"` // YYYY-MM
}

func (DistributePoolArgs) Kind() string { return "distribute_pool" }

func (DistributePoolArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type Queue struct {
	Client *river.Client[pgx.Tx]
	pool   *pgxpool.Pool
}

type Config struct {
	Workers *river.Workers
}

func Start(ctx context.Context, workers *river.Workers) (*Queue, error) {
	url := viper.GetString("database.url")
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("queue pool: %w", err)
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("river migrate init: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		pool.Close()
		return nil, fmt.Errorf("river migrate: %w", err)
	}

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 4},
		},
		Workers:          workers,
		WorkerMiddleware: nil,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("river client: %w", err)
	}
	if err := client.Start(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("river start: %w", err)
	}
	slog.Info("river workers iniciados")
	return &Queue{Client: client, pool: pool}, nil
}

func (q *Queue) Stop(ctx context.Context) error {
	if q == nil || q.Client == nil {
		return nil
	}
	timeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := q.Client.Stop(timeout); err != nil {
		return err
	}
	q.pool.Close()
	return nil
}

func (q *Queue) EnqueueValidate(ctx context.Context, contributionID uuid.UUID) error {
	_, err := q.Client.Insert(ctx, ValidateContributionArgs{ContributionID: contributionID}, nil)
	return err
}

func (q *Queue) EnqueueAggregate(ctx context.Context, cycleID uuid.UUID) error {
	_, err := q.Client.Insert(ctx, AggregateCycleArgs{CycleID: cycleID}, nil)
	return err
}

func (q *Queue) EnqueueDistribute(ctx context.Context, month string) error {
	_, err := q.Client.Insert(ctx, DistributePoolArgs{Month: month}, nil)
	return err
}
