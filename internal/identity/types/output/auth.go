package output

import (
	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/identity/domain"
)

type MFAChallenge struct {
	MFARequired bool      `json:"mfa_required"`
	MFAToken    string    `json:"mfa_token"`
	UserID      uuid.UUID `json:"user_id"`
}

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func NewTokens(access, refresh string) Tokens {
	return Tokens{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    auth.AccessTTLSeconds(),
		TokenType:    "Bearer",
	}
}

type Me struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Role       string    `json:"role"`
	MFAEnabled bool      `json:"mfa_enabled"`
	Status     string    `json:"status"`
}

func NewMe(u domain.User) Me {
	return Me{
		ID:         u.ID,
		Email:      u.Email,
		Phone:      u.Phone,
		Role:       string(u.Role),
		MFAEnabled: u.MFAEnabled,
		Status:     string(u.Status),
	}
}

type PendingOTP struct {
	UserID  uuid.UUID `json:"user_id"`
	Code    string    `json:"code"`
	Purpose string    `json:"purpose"`
}
