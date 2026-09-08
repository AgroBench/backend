package domain

import (
	"time"

	"github.com/google/uuid"

	coredomain "github.com/AgroBench/backend/internal/core/domain"
)

type Status string

const (
	StatusCommitted  Status = "committed"
	StatusRevealed   Status = "revealed"
	StatusValidating Status = "validating"
	StatusAccepted   Status = "accepted"
	StatusRejected   Status = "rejected"
)

type Contribution struct {
	ID           uuid.UUID
	CycleID      uuid.UUID
	WalletID     uuid.UUID
	PropertyID   uuid.UUID
	Level        coredomain.ContributionLevel
	Attempt      int
	CommitHash   string
	CommitTx     string
	CommitAt     time.Time
	Ciphertext   []byte
	RevealAt     *time.Time
	Status       Status
	RejectReason string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type StakeStatus string

const (
	StakeLocked   StakeStatus = "locked"
	StakeReleased StakeStatus = "released"
)

type Stake struct {
	ID             uuid.UUID
	ContributionID uuid.UUID
	AmountUSDC     float64
	LockTx         string
	ReleaseTx      string
	Status         StakeStatus
}

type Verdict struct {
	ID               uuid.UUID
	ContributionID   uuid.UUID
	Verdict          string
	Checks           []byte
	EnclaveSignature []byte
	EnclavePubkey    string
}

type ValidatedMetric struct {
	ID             uuid.UUID
	ContributionID uuid.UUID
	Metric         string
	Value          float64
}

type Attestation struct {
	ID             uuid.UUID
	ContributionID uuid.UUID
	Tx             string
	RewardUSDC     float64
	PaidAt         time.Time
}
