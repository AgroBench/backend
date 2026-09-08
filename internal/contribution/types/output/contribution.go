package output

import (
	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/contribution/domain"
)

type Contribution struct {
	ID           uuid.UUID `json:"id"`
	CycleID      uuid.UUID `json:"cycle_id"`
	WalletID     uuid.UUID `json:"wallet_id"`
	Level        string    `json:"level"`
	Attempt      int       `json:"attempt"`
	CommitHash   string    `json:"commit_hash"`
	CommitTx     string    `json:"commit_tx,omitempty"`
	Status       string    `json:"status"`
	RejectReason string    `json:"reject_reason,omitempty"`
}

func New(c domain.Contribution) Contribution {
	return Contribution{
		ID: c.ID, CycleID: c.CycleID, WalletID: c.WalletID,
		Level: string(c.Level), Attempt: c.Attempt,
		CommitHash: c.CommitHash, CommitTx: c.CommitTx,
		Status: string(c.Status), RejectReason: c.RejectReason,
	}
}

type EnclaveKey struct {
	BoxPublicKey     string `json:"box_public_key"`
	SigningPublicKey string `json:"signing_public_key"`
	Provider         string `json:"provider"`
	Attestation      []byte `json:"attestation,omitempty"`
}
