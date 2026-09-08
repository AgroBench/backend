package domain

import (
	"time"

	"github.com/google/uuid"
)

type Wallet struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Pubkey        string
	EncryptedBlob []byte
	BlobVersion   int
	ExportedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
