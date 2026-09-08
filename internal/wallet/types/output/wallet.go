package output

import (
	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/domain"
)

type Wallet struct {
	ID          uuid.UUID `json:"id"`
	Pubkey      string    `json:"pubkey"`
	BlobVersion int       `json:"blob_version"`
	BalanceUSDC float64   `json:"balance_usdc"`
	RewardUSDC  float64   `json:"reward_usdc"`
	ExportedAt  *string   `json:"exported_at,omitempty"`
}

func NewWallet(id uuid.UUID, pubkey string, version int, balance, reward domain.MicroUSDC) Wallet {
	return Wallet{
		ID:          id,
		Pubkey:      pubkey,
		BlobVersion: version,
		BalanceUSDC: balance.Float(),
		RewardUSDC:  reward.Float(),
	}
}

type Export struct {
	Pubkey        string `json:"pubkey"`
	EncryptedBlob string `json:"encrypted_blob"`
	BlobVersion   int    `json:"blob_version"`
}
