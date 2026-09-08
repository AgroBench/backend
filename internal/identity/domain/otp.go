package domain

import (
	"time"

	"github.com/google/uuid"
)

type OTPPurpose string

const (
	OTPLogin    OTPPurpose = "login"
	OTPRecovery OTPPurpose = "recovery"
)

type OTPCode struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Purpose    OTPPurpose
	CodeHash   string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	Attempts   int
	CreatedAt  time.Time
}

func (o OTPCode) Expired(now time.Time) bool {
	return !now.Before(o.ExpiresAt)
}

func (o OTPCode) Consumed() bool { return o.ConsumedAt != nil }
