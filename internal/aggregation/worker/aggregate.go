package worker

import (
	"context"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/riverqueue/river"
	"github.com/spf13/viper"

	coredomain "github.com/AgroBench/backend/internal/core/domain"
	cyclerepo "github.com/AgroBench/backend/internal/cycle/repository"
	"github.com/AgroBench/backend/pkg/adapter/queue"
)

type Aggregate struct {
	river.WorkerDefaults[queue.AggregateCycleArgs]
	db *sqlx.DB
}

func NewAggregate(db *sqlx.DB) *Aggregate { return &Aggregate{db: db} }

func (w *Aggregate) Work(ctx context.Context, job *river.Job[queue.AggregateCycleArgs]) error {
	cycleID := job.Args.CycleID
	type row struct {
		Level  string    `db:"level"`
		Metric string    `db:"metric"`
		Value  float64   `db:"value"`
		Wallet uuid.UUID `db:"wallet_id"`
	}
	var rows []row
	if err := sqlx.SelectContext(ctx, w.db, &rows, `
		SELECT c.level, m.metric, m.value, c.wallet_id
		  FROM validated_metrics m
		  JOIN contributions c ON c.id = m.contribution_id
		 WHERE c.cycle_id = $1 AND c.status = 'accepted'`, cycleID); err != nil {
		return err
	}

	type key struct{ level, metric string }
	groups := map[key][]float64{}
	walletsByLevel := map[string]map[uuid.UUID]struct{}{}
	for _, r := range rows {
		k := key{r.Level, r.Metric}
		groups[k] = append(groups[k], r.Value)
		if walletsByLevel[r.Level] == nil {
			walletsByLevel[r.Level] = map[uuid.UUID]struct{}{}
		}
		walletsByLevel[r.Level][r.Wallet] = struct{}{}
	}
	for k, vals := range groups {
		mean, median, p25, p75 := stats(vals)
		_, err := w.db.ExecContext(ctx, `
			INSERT INTO aggregates (id, cycle_id, level, metric, mean, median, p25, p75, n)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			coredomain.NewID(), cycleID, k.level, k.metric, mean, median, p25, p75, len(vals))
		if err != nil {
			return err
		}
	}

	mult := map[string]float64{
		"basic":        viper.GetFloat64("rewards.level_multipliers.basic"),
		"intermediate": viper.GetFloat64("rewards.level_multipliers.intermediate"),
		"advanced":     viper.GetFloat64("rewards.level_multipliers.advanced"),
	}
	if mult["basic"] == 0 {
		mult["basic"] = 1
		mult["intermediate"] = 2
		mult["advanced"] = 3.5
	}
	type acc struct {
		wallet uuid.UUID
		weight float64
	}
	var accs []acc
	var total float64
	seen := map[uuid.UUID]float64{}
	for _, r := range rows {
		seen[r.Wallet] = mult[r.Level]
	}
	n := float64(len(seen))
	if n == 0 {
		n = 1
	}
	for wal, m := range seen {
		w := m / n
		accs = append(accs, acc{wal, w})
		total += w
	}
	for _, a := range accs {
		weight := a.weight
		if total > 0 {
			weight = a.weight / total
		}
		_, err := w.db.ExecContext(ctx, `
			INSERT INTO contribution_weights (id, cycle_id, wallet_id, weight)
			VALUES ($1,$2,$3,$4)`,
			coredomain.NewID(), cycleID, a.wallet, weight)
		if err != nil {
			return err
		}
	}
	return cyclerepo.New(w.db).MarkAggregated(ctx, cycleID)
}

func stats(vals []float64) (mean, median, p25, p75 float64) {
	if len(vals) == 0 {
		return
	}
	sort.Float64s(vals)
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean = sum / float64(len(vals))
	median = percentile(vals, 50)
	p25 = percentile(vals, 25)
	p75 = percentile(vals, 75)
	return
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := (p / 100) * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	w := idx - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}
