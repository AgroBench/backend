package contract

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/contribution/domain"
)

type Repo interface {
	Create(ctx context.Context, c domain.Contribution) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Contribution, error)
	ListByWallet(ctx context.Context, walletID uuid.UUID) ([]domain.Contribution, error)
	ActiveInCycle(ctx context.Context, cycleID, walletID uuid.UUID) (domain.Contribution, error)
	HasRejected(ctx context.Context, cycleID, walletID uuid.UUID) (bool, error)
	UpdateReveal(ctx context.Context, id uuid.UUID, ciphertext []byte) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, reason string) error
	CreateStake(ctx context.Context, s domain.Stake) error
	GetLockedStakeByWallet(ctx context.Context, walletID uuid.UUID) (domain.Stake, error)
	GetStakeByContribution(ctx context.Context, contributionID uuid.UUID) (domain.Stake, error)
	ReleaseStake(ctx context.Context, id uuid.UUID, tx string) error
	SaveVerdict(ctx context.Context, v domain.Verdict, metrics []domain.ValidatedMetric) error
	SaveAttestation(ctx context.Context, a domain.Attestation) error
	ConsecutiveAccepted(ctx context.Context, walletID, cultureID, regionID uuid.UUID) (int, error)
	WithTx(ctx context.Context, fn func(Repo) error) error
}
