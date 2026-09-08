package domain

import (
	"time"

	"github.com/google/uuid"

	coredomain "github.com/AgroBench/backend/internal/core/domain"
)

type UserStatus string

const (
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

type User struct {
	ID           uuid.UUID
	Email        string
	Phone        string
	CPFHMAC      string
	PasswordHash string
	Role         coredomain.Role
	MFAEnabled   bool
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u User) IsActive() bool { return u.Status == UserActive }
