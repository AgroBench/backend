package usecase

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	crepo "github.com/AgroBench/backend/internal/contribution/repository"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	cyclerepo "github.com/AgroBench/backend/internal/cycle/repository"
	walletrepo "github.com/AgroBench/backend/internal/wallet/repository"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
)

type MetricPoint struct {
	Metric string  `json:"metric"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	P25    float64 `json:"p25"`
	P75    float64 `json:"p75"`
	N      int     `json:"n"`
}

type ProducerPanel struct {
	CycleID uuid.UUID     `json:"cycle_id"`
	Level   string        `json:"level"`
	Metrics []MetricPoint `json:"metrics"`
}

type PanelBlocked struct {
	Code            string `json:"code"`
	Message         string `json:"message"`
	CyclesValidated int    `json:"cycles_validated"`
	CyclesRequired  int    `json:"cycles_required"`
}

type Me struct{ db *sqlx.DB }

func NewMe(db *sqlx.DB) *Me { return &Me{db: db} }

func (u *Me) Execute(ctx context.Context, userID, cycleID uuid.UUID) (ProducerPanel, error) {
	const op = "benchmark.Me"
	wallets := walletrepo.New(u.db)
	w, err := wallets.GetByUserID(ctx, userID)
	if err != nil {
		return ProducerPanel{}, err
	}
	cycle, err := cyclerepo.New(u.db).GetByID(ctx, cycleID)
	if err != nil {
		return ProducerPanel{}, err
	}
	n, err := crepo.New(u.db).ConsecutiveAccepted(ctx, w.ID, cycle.CultureID, cycle.MicroRegionID)
	if err != nil {
		return ProducerPanel{}, err
	}
	need := viper.GetInt("benchmark.free_after_cycles")
	if need <= 0 {
		need = 3
	}
	if n < need {
		return ProducerPanel{}, apperrors.Forbidden(op, errors.New("not enough")).WithDetail(
			"painel bloqueado: " + strconv.Itoa(n) + "/" + strconv.Itoa(need) + " ciclos consecutivos")
	}
	var rows []struct {
		Metric string  `db:"metric"`
		Mean   float64 `db:"mean"`
		Median float64 `db:"median"`
		P25    float64 `db:"p25"`
		P75    float64 `db:"p75"`
		N      int     `db:"n"`
	}
	err = sqlx.SelectContext(ctx, u.db, &rows, `
		SELECT metric, mean, median, p25, p75, n
		  FROM aggregates
		 WHERE cycle_id = $1 AND level = 'basic'
		 ORDER BY metric`, cycleID)
	if err != nil {
		return ProducerPanel{}, apperrors.FromDBError(op, err)
	}
	out := ProducerPanel{CycleID: cycleID, Level: "basic"}
	for _, r := range rows {
		out.Metrics = append(out.Metrics, MetricPoint{r.Metric, r.Mean, r.Median, r.P25, r.P75, r.N})
	}
	return out, nil
}

type Report struct{ db *sqlx.DB }

func NewReport(db *sqlx.DB) *Report { return &Report{db: db} }

type InstitutionReport struct {
	CycleID uuid.UUID     `json:"cycle_id"`
	Metrics []MetricPoint `json:"metrics"`
}

func (u *Report) Execute(ctx context.Context, institutionID, cycleID uuid.UUID) (InstitutionReport, error) {
	const op = "benchmark.Report"
	var rows []struct {
		Metric string  `db:"metric"`
		Mean   float64 `db:"mean"`
		Median float64 `db:"median"`
		P25    float64 `db:"p25"`
		P75    float64 `db:"p75"`
		N      int     `db:"n"`
		Level  string  `db:"level"`
	}
	err := sqlx.SelectContext(ctx, u.db, &rows, `
		SELECT metric, mean, median, p25, p75, n, level
		  FROM aggregates WHERE cycle_id = $1 ORDER BY level, metric`, cycleID)
	if err != nil {
		return InstitutionReport{}, apperrors.FromDBError(op, err)
	}
	out := InstitutionReport{CycleID: cycleID}
	for _, r := range rows {
		out.Metrics = append(out.Metrics, MetricPoint{r.Level + ":" + r.Metric, r.Mean, r.Median, r.P25, r.P75, r.N})
	}
	_, _ = u.db.ExecContext(ctx, `
		INSERT INTO report_access (id, institution_id, subscription_id, cycle_id, accessed_at)
		SELECT $1, $2, s.id, $3, now()
		  FROM subscriptions s
		 WHERE s.institution_id = $2 AND s.status = 'active'
		 ORDER BY s.period_end DESC LIMIT 1`, coredomain.NewID(), institutionID, cycleID)
	return out, nil
}
