package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/spf13/viper"

	carrepo "github.com/AgroBench/backend/internal/car/repository"
	"github.com/AgroBench/backend/internal/contribution/domain"
	crepo "github.com/AgroBench/backend/internal/contribution/repository"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	cyclerepo "github.com/AgroBench/backend/internal/cycle/repository"
	walletrepo "github.com/AgroBench/backend/internal/wallet/repository"
	"github.com/AgroBench/backend/pkg/adapter/queue"
	"github.com/AgroBench/backend/pkg/port"
	"github.com/jmoiron/sqlx"
)

type Validate struct {
	river.WorkerDefaults[queue.ValidateContributionArgs]
	db      *sqlx.DB
	enclave port.Enclave
	chain   port.ChainClient
	ref     port.ReferenceDataClient
}

func NewValidate(db *sqlx.DB, enc port.Enclave, chain port.ChainClient, ref port.ReferenceDataClient) *Validate {
	return &Validate{db: db, enclave: enc, chain: chain, ref: ref}
}

func (w *Validate) Work(ctx context.Context, job *river.Job[queue.ValidateContributionArgs]) error {
	id := job.Args.ContributionID
	repo := crepo.New(w.db)
	cycles := cyclerepo.New(w.db)
	catalog := carrepo.NewCatalog(w.db)
	wallets := walletrepo.New(w.db)

	c, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if c.Status == domain.StatusAccepted || c.Status == domain.StatusRejected {
		return nil
	}
	_ = repo.UpdateStatus(ctx, c.ID, domain.StatusValidating, "")

	cycle, err := cycles.GetByID(ctx, c.CycleID)
	if err != nil {
		return err
	}
	culture, err := catalog.GetCulture(ctx, cycle.CultureID)
	if err != nil {
		return err
	}
	region, err := catalog.GetMicroRegion(ctx, cycle.MicroRegionID)
	if err != nil {
		return err
	}
	refs, err := w.ref.ExpectedRanges(ctx, culture.Code, region.IBGECode)
	if err != nil {
		return err
	}

	verdict, err := w.enclave.Validate(ctx, port.ValidateRequest{
		ContributionID: c.ID, CycleID: c.CycleID, Level: c.Level,
		CommitHash: c.CommitHash, Ciphertext: c.Ciphertext, References: refs,
	})
	if err != nil {
		return err
	}

	checks, _ := json.Marshal(verdict.Checks)
	v := domain.Verdict{
		ID: coredomain.NewID(), ContributionID: c.ID,
		Checks: checks, EnclaveSignature: verdict.Signature, EnclavePubkey: verdict.SignerPubKey,
	}
	var metrics []domain.ValidatedMetric
	if verdict.Accepted {
		v.Verdict = "accepted"
		for _, m := range verdict.Metrics {
			metrics = append(metrics, domain.ValidatedMetric{
				ID: coredomain.NewID(), ContributionID: c.ID, Metric: m.Name, Value: m.Value,
			})
		}
	} else {
		v.Verdict = "rejected"
	}
	if err := repo.SaveVerdict(ctx, v, metrics); err != nil {
		return err
	}

	wallet, err := wallets.GetByID(ctx, c.WalletID)
	if err != nil {
		return err
	}

	if !verdict.Accepted {
		_ = repo.UpdateStatus(ctx, c.ID, domain.StatusRejected, "validação rejeitada")
		slog.Info("contribuição rejeitada", "id", c.ID)
		return nil
	}

	mult := viper.GetFloat64("rewards.level_multipliers." + string(c.Level))
	if mult <= 0 {
		mult = 1
	}
	reward := coredomain.USDC(viper.GetFloat64("rewards.base_usdc")).MulFloat(mult)

	attestTx, err := w.chain.Attest(ctx, port.AttestRequest{
		Wallet: port.Account(wallet.Pubkey), ContributionID: c.ID, CycleID: c.CycleID,
		Hash: c.CommitHash, Level: c.Level, Accepted: true,
	})
	if err != nil {
		return err
	}
	paid := reward.Float()
	rewardTx := string(attestTx)
	if ref, err := w.chain.TransferUSDC(ctx, port.TransferRequest{
		From: w.chain.Treasury(), To: port.Account(wallet.Pubkey), Amount: reward, Memo: "reward:" + c.ID.String(),
	}); err != nil {
		slog.Warn("recompensa USDC não enviada; contribuição segue aceita", "id", c.ID, "err", err)
		paid = 0
	} else {
		rewardTx = string(ref)
	}
	if err := repo.SaveAttestation(ctx, domain.Attestation{
		ID: coredomain.NewID(), ContributionID: c.ID, Tx: rewardTx, RewardUSDC: paid, PaidAt: time.Now(),
	}); err != nil {
		return err
	}
	if err := repo.UpdateStatus(ctx, c.ID, domain.StatusAccepted, ""); err != nil {
		return err
	}

	n, err := repo.ConsecutiveAccepted(ctx, c.WalletID, cycle.CultureID, cycle.MicroRegionID)
	if err != nil {
		return err
	}
	need := viper.GetInt("stake.release_after_cycles")
	if need <= 0 {
		need = 2
	}
	if n >= need {
		if st, err := repo.GetLockedStakeByWallet(ctx, c.WalletID); err == nil {
			rel, err := w.chain.ReleaseStake(ctx, port.StakeRequest{
				Wallet: port.Account(wallet.Pubkey), ContributionID: st.ContributionID, Amount: coredomain.USDC(st.AmountUSDC),
			})
			if err == nil {
				_ = repo.ReleaseStake(ctx, st.ID, string(rel))
			} else if errors.Is(err, port.ErrNeedsCoSign) {
				slog.Info("release de stake exige assinatura do produtor",
					"contribution_id", st.ContributionID,
					"hint", "POST /api/v1/contributions/{id}/release-stake/tx")
			} else {
				slog.Warn("falha ao liberar stake", "err", err)
			}
		}
	}
	slog.Info("contribuição aceita", "id", c.ID, "reward_usdc", reward.Float())
	_ = uuid.Nil
	return nil
}
