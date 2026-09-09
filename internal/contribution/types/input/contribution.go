package input

import "github.com/google/uuid"

type Commit struct {
	CycleID    uuid.UUID `json:"cycle_id"    validate:"required"`
	Level      string    `json:"level"       validate:"required,oneof=basic intermediate advanced"`
	Hash       string    `json:"hash"        validate:"required,len=64"`
	PropertyID uuid.UUID `json:"property_id" validate:"required"`
	// StakeTx é a signature (base58) já confirmada de POST /chain/stake/lock-submit.
	StakeTx string `json:"stake_tx,omitempty"`
	// SignedTx é a tx lock_stake em base64 já assinada pelo produtor (alternativa a stake_tx).
	SignedTx string `json:"signed_tx,omitempty"`
}

type Reveal struct {
	Ciphertext string `json:"ciphertext" validate:"required"` // base64
}

type LockStakeTx struct {
	Amount int64 `json:"amount"` // micro-USDC; 0 = stake.amount_usdc da config (10_000_000)
}

type SubmitSignedTx struct {
	Tx string `json:"tx" validate:"required"`
}
