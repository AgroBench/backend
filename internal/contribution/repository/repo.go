package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/contribution/contract"
	"github.com/AgroBench/backend/internal/contribution/domain"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/adapter/database"
)

type row struct {
	ID           uuid.UUID  `db:"id"`
	CycleID      uuid.UUID  `db:"cycle_id"`
	WalletID     uuid.UUID  `db:"wallet_id"`
	PropertyID   uuid.UUID  `db:"property_id"`
	Level        string     `db:"level"`
	Attempt      int        `db:"attempt"`
	CommitHash   string     `db:"commit_hash"`
	CommitTx     string     `db:"commit_tx"`
	CommitAt     time.Time  `db:"commit_at"`
	Ciphertext   []byte     `db:"ciphertext"`
	RevealAt     *time.Time `db:"reveal_at"`
	Status       string     `db:"status"`
	RejectReason *string    `db:"reject_reason"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

func (r row) toDomain() domain.Contribution {
	reason := ""
	if r.RejectReason != nil {
		reason = *r.RejectReason
	}
	return domain.Contribution{
		ID: r.ID, CycleID: r.CycleID, WalletID: r.WalletID, PropertyID: r.PropertyID,
		Level: coredomain.ContributionLevel(r.Level), Attempt: r.Attempt,
		CommitHash: r.CommitHash, CommitTx: r.CommitTx, CommitAt: r.CommitAt,
		Ciphertext: r.Ciphertext, RevealAt: r.RevealAt, Status: domain.Status(r.Status),
		RejectReason: reason, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

const cols = `id, cycle_id, wallet_id, property_id, level, attempt, commit_hash, commit_tx, commit_at, ciphertext, reveal_at, status, reject_reason, created_at, updated_at`

type Repo struct{ db sqlx.ExtContext }

func New(db sqlx.ExtContext) *Repo { return &Repo{db: db} }

func (r *Repo) WithTx(ctx context.Context, fn func(contract.Repo) error) error {
	db, ok := r.db.(*sqlx.DB)
	if !ok {
		return apperrors.Internal("contribution.WithTx", fmt.Errorf("conexão não transacional"))
	}
	return database.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		return fn(New(tx))
	})
}

func (r *Repo) Create(ctx context.Context, c domain.Contribution) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO contributions (id, cycle_id, wallet_id, property_id, level, attempt, commit_hash, commit_tx, commit_at, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		c.ID, c.CycleID, c.WalletID, c.PropertyID, c.Level, c.Attempt, c.CommitHash, c.CommitTx, c.CommitAt, c.Status)
	if err != nil {
		return apperrors.FromDBError("contribution.Create", err)
	}
	return nil
}

func (r *Repo) get(ctx context.Context, op, q string, args ...any) (domain.Contribution, error) {
	var rec row
	err := sqlx.GetContext(ctx, r.db, &rec, q, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Contribution{}, apperrors.NotFound(op, err).WithDetail("contribuição não encontrada")
	}
	if err != nil {
		return domain.Contribution{}, apperrors.FromDBError(op, err)
	}
	return rec.toDomain(), nil
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (domain.Contribution, error) {
	return r.get(ctx, "contribution.GetByID", `SELECT `+cols+` FROM contributions WHERE id = $1`, id)
}

func (r *Repo) ListByWallet(ctx context.Context, walletID uuid.UUID) ([]domain.Contribution, error) {
	var rows []row
	err := sqlx.SelectContext(ctx, r.db, &rows, `SELECT `+cols+` FROM contributions WHERE wallet_id = $1 ORDER BY created_at DESC`, walletID)
	if err != nil {
		return nil, apperrors.FromDBError("contribution.ListByWallet", err)
	}
	out := make([]domain.Contribution, len(rows))
	for i, rec := range rows {
		out[i] = rec.toDomain()
	}
	return out, nil
}

func (r *Repo) ActiveInCycle(ctx context.Context, cycleID, walletID uuid.UUID) (domain.Contribution, error) {
	return r.get(ctx, "contribution.ActiveInCycle",
		`SELECT `+cols+` FROM contributions WHERE cycle_id = $1 AND wallet_id = $2 AND status <> 'rejected' LIMIT 1`,
		cycleID, walletID)
}

func (r *Repo) HasRejected(ctx context.Context, cycleID, walletID uuid.UUID) (bool, error) {
	var n int
	err := sqlx.GetContext(ctx, r.db, &n, `
		SELECT COUNT(*) FROM contributions WHERE cycle_id = $1 AND wallet_id = $2 AND status = 'rejected'`,
		cycleID, walletID)
	return n > 0, err
}

func (r *Repo) UpdateReveal(ctx context.Context, id uuid.UUID, ciphertext []byte) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE contributions SET ciphertext = $2, reveal_at = now(), status = 'revealed' WHERE id = $1 AND status = 'committed'`,
		id, ciphertext)
	if err != nil {
		return apperrors.FromDBError("contribution.UpdateReveal", err)
	}
	return nil
}

func (r *Repo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, reason string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE contributions SET status = $2, reject_reason = $3 WHERE id = $1`, id, status, nullIfEmpty(reason))
	if err != nil {
		return apperrors.FromDBError("contribution.UpdateStatus", err)
	}
	return nil
}

func (r *Repo) CreateStake(ctx context.Context, s domain.Stake) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stakes (id, contribution_id, amount_usdc, lock_tx, status) VALUES ($1,$2,$3,$4,$5)`,
		s.ID, s.ContributionID, s.AmountUSDC, s.LockTx, s.Status)
	if err != nil {
		return apperrors.FromDBError("contribution.CreateStake", err)
	}
	return nil
}

func (r *Repo) GetLockedStakeByWallet(ctx context.Context, walletID uuid.UUID) (domain.Stake, error) {
	var s struct {
		ID             uuid.UUID `db:"id"`
		ContributionID uuid.UUID `db:"contribution_id"`
		AmountUSDC     float64   `db:"amount_usdc"`
		LockTx         string    `db:"lock_tx"`
		Status         string    `db:"status"`
	}
	err := sqlx.GetContext(ctx, r.db, &s, `
		SELECT s.id, s.contribution_id, s.amount_usdc, s.lock_tx, s.status
		  FROM stakes s
		  JOIN contributions c ON c.id = s.contribution_id
		 WHERE c.wallet_id = $1 AND s.status = 'locked'
		 ORDER BY s.created_at ASC LIMIT 1`, walletID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Stake{}, apperrors.NotFound("contribution.GetLockedStake", err)
	}
	if err != nil {
		return domain.Stake{}, apperrors.FromDBError("contribution.GetLockedStake", err)
	}
	return domain.Stake{ID: s.ID, ContributionID: s.ContributionID, AmountUSDC: s.AmountUSDC, LockTx: s.LockTx, Status: domain.StakeStatus(s.Status)}, nil
}

func (r *Repo) ReleaseStake(ctx context.Context, id uuid.UUID, tx string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE stakes SET status = 'released', release_tx = $2 WHERE id = $1`, id, tx)
	if err != nil {
		return apperrors.FromDBError("contribution.ReleaseStake", err)
	}
	return nil
}

func (r *Repo) SaveVerdict(ctx context.Context, v domain.Verdict, metrics []domain.ValidatedMetric) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO validation_verdicts (id, contribution_id, verdict, checks, enclave_signature, enclave_pubkey)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		v.ID, v.ContributionID, v.Verdict, v.Checks, v.EnclaveSignature, v.EnclavePubkey)
	if err != nil {
		return apperrors.FromDBError("contribution.SaveVerdict", err)
	}
	for _, m := range metrics {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO validated_metrics (id, contribution_id, metric, value) VALUES ($1,$2,$3,$4)`,
			m.ID, m.ContributionID, m.Metric, m.Value)
		if err != nil {
			return apperrors.FromDBError("contribution.SaveMetric", err)
		}
	}
	return nil
}

func (r *Repo) SaveAttestation(ctx context.Context, a domain.Attestation) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO attestations (id, contribution_id, tx, reward_usdc, paid_at)
		VALUES ($1,$2,$3,$4,$5)`,
		a.ID, a.ContributionID, a.Tx, a.RewardUSDC, a.PaidAt)
	if err != nil {
		return apperrors.FromDBError("contribution.SaveAttestation", err)
	}
	return nil
}

func (r *Repo) ConsecutiveAccepted(ctx context.Context, walletID, cultureID, regionID uuid.UUID) (int, error) {
	type st struct {
		Status string `db:"status"`
	}
	var list []st
	err := sqlx.SelectContext(ctx, r.db, &list, `
		SELECT c.status
		  FROM contributions c
		  JOIN cycles cy ON cy.id = c.cycle_id
		 WHERE c.wallet_id = $1 AND cy.culture_id = $2 AND cy.micro_region_id = $3
		   AND c.status IN ('accepted','rejected')
		 ORDER BY cy.opens_at DESC`, walletID, cultureID, regionID)
	if err != nil {
		return 0, apperrors.FromDBError("contribution.ConsecutiveAccepted", err)
	}
	count := 0
	for _, s := range list {
		if s.Status != "accepted" {
			break
		}
		count++
	}
	return count, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
