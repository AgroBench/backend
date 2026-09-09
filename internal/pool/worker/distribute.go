package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/riverqueue/river"
	"github.com/spf13/viper"

	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/adapter/queue"
	"github.com/AgroBench/backend/pkg/port"
)

type Distribute struct {
	river.WorkerDefaults[queue.DistributePoolArgs]
	db    *sqlx.DB
	chain port.ChainClient
}

func NewDistribute(db *sqlx.DB, chain port.ChainClient) *Distribute {
	return &Distribute{db: db, chain: chain}
}

func (w *Distribute) Work(ctx context.Context, job *river.Job[queue.DistributePoolArgs]) error {
	month := job.Args.Month
	if month == "" {
		month = time.Now().AddDate(0, -1, 0).Format("2006-01")
	}
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return err
	}
	end := start.AddDate(0, 1, 0)

	var gross float64
	_ = sqlx.GetContext(ctx, w.db, &gross, `
		SELECT COALESCE(SUM(amount), 0) FROM payments
		 WHERE confirmed_at >= $1 AND confirmed_at < $2`, start, end)

	var contribs int
	_ = sqlx.GetContext(ctx, w.db, &contribs, `
		SELECT COUNT(*) FROM contributions
		 WHERE status = 'accepted' AND updated_at >= $1 AND updated_at < $2`, start, end)

	infraEach := viper.GetFloat64("pool.infra_cost_per_contribution_usdc")
	feePct := viper.GetFloat64("pool.maintainer_fee_pct")
	infra := infraEach * float64(contribs)
	fee := gross * feePct / 100
	net := gross - infra - fee
	if net < 0 {
		net = 0
	}

	periodID := coredomain.NewID()
	status := "distributed"
	if net == 0 {
		status = "carried"
	}

	type acc struct {
		CycleID uuid.UUID `db:"cycle_id"`
		N       int       `db:"n"`
	}
	var accesses []acc
	_ = sqlx.SelectContext(ctx, w.db, &accesses, `
		SELECT cycle_id, COUNT(*) AS n FROM report_access
		 WHERE accessed_at >= $1 AND accessed_at < $2
		 GROUP BY cycle_id`, start, end)
	totalAcc := 0
	for _, a := range accesses {
		totalAcc += a.N
	}
	if totalAcc == 0 {
		status = "carried"
	}

	_, err = w.db.ExecContext(ctx, `
		INSERT INTO pool_periods (id, month, gross, infra_cost, maintainer_fee, net, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (month) DO UPDATE SET gross = EXCLUDED.gross, infra_cost = EXCLUDED.infra_cost,
		  maintainer_fee = EXCLUDED.maintainer_fee, net = EXCLUDED.net, status = EXCLUDED.status`,
		periodID, month, gross, infra, fee, net, status)
	if err != nil {
		return err
	}
	_ = sqlx.GetContext(ctx, w.db, &periodID, `SELECT id FROM pool_periods WHERE month = $1`, month)

	if totalAcc == 0 || net <= 0 {
		slog.Info("pool sem distribuição", "month", month, "net", net)
		return nil
	}

	type pendingPayout struct {
		WalletID uuid.UUID
		CycleID  uuid.UUID
		Pubkey   port.Account
		Amount   coredomain.MicroUSDC
	}
	var pending []pendingPayout

	for _, a := range accesses {
		slice := net * float64(a.N) / float64(totalAcc)
		type wt struct {
			Wallet uuid.UUID `db:"wallet_id"`
			Weight float64   `db:"weight"`
		}
		var weights []wt
		_ = sqlx.SelectContext(ctx, w.db, &weights, `
			SELECT wallet_id, weight FROM contribution_weights WHERE cycle_id = $1`, a.CycleID)
		var wsum float64
		for _, x := range weights {
			wsum += x.Weight
		}
		if wsum == 0 {
			continue
		}
		for _, x := range weights {
			amt := coredomain.USDC(slice * x.Weight / wsum)
			if amt <= 0 {
				continue
			}
			pending = append(pending, pendingPayout{
				WalletID: x.Wallet, CycleID: a.CycleID,
				Pubkey: walletPubkey(ctx, w.db, x.Wallet), Amount: amt,
			})
		}
	}

	useProgram := w.chain.ProgramID() != ""
	var batchRef port.TxRef
	if useProgram && len(pending) > 0 {
		payouts := make([]port.PoolPayout, 0, len(pending))
		for _, p := range pending {
			payouts = append(payouts, port.PoolPayout{Wallet: p.Pubkey, Amount: p.Amount})
		}
		ref, err := w.chain.DistributePool(ctx, payouts)
		if err != nil {
			return err
		}
		batchRef = ref
	}

	for _, p := range pending {
		tx := batchRef
		var err error
		if !useProgram {
			tx, err = w.chain.TransferUSDC(ctx, port.TransferRequest{
				From: w.chain.Pool(), To: p.Pubkey, Amount: p.Amount, Memo: "payout:" + periodID.String(),
			})
			if err != nil {
				slog.Warn("payout falhou", "wallet", p.WalletID, "err", err)
				continue
			}
		}
		_, _ = w.db.ExecContext(ctx, `
			INSERT INTO payouts (id, pool_period_id, wallet_id, cycle_id, amount_usdc, tx)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			coredomain.NewID(), periodID, p.WalletID, p.CycleID, p.Amount.Float(), string(tx))
	}
	slog.Info("pool distribuído", "month", month, "net", net)
	return nil
}

func walletPubkey(ctx context.Context, db *sqlx.DB, id uuid.UUID) port.Account {
	var pk string
	_ = sqlx.GetContext(ctx, db, &pk, `SELECT pubkey FROM wallets WHERE id = $1`, id)
	return port.Account(pk)
}
