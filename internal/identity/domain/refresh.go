package domain

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	Device    string
	CreatedAt time.Time
}

func (t RefreshToken) Revoked() bool { return t.RevokedAt != nil }

func (t RefreshToken) Expired(now time.Time) bool {
	return !now.Before(t.ExpiresAt)
}
