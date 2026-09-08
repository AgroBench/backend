package contract

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/identity/domain"
	"github.com/AgroBench/backend/internal/identity/types/input"
	"github.com/AgroBench/backend/internal/identity/types/output"
)

type UserWriter interface {
	Create(ctx context.Context, u domain.User) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
}

type UserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByCPFHMAC(ctx context.Context, hmac string) (domain.User, error)
}

type OTPWriter interface {
	Create(ctx context.Context, o domain.OTPCode) error
	InvalidatePending(ctx context.Context, userID uuid.UUID, purpose domain.OTPPurpose) error
	IncrementAttempts(ctx context.Context, id uuid.UUID) error
	Consume(ctx context.Context, id uuid.UUID) error
}

type OTPReader interface {
	GetPending(ctx context.Context, userID uuid.UUID, purpose domain.OTPPurpose) (domain.OTPCode, error)
}

type RefreshWriter interface {
	Create(ctx context.Context, t domain.RefreshToken) error
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

type RefreshReader interface {
	GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
}

type WalletPubkeyFinder interface {
	GetPubkeyByUserID(ctx context.Context, userID uuid.UUID) (string, error)
}

type Register interface {
	Execute(ctx context.Context, in input.Register) (output.MFAChallenge, error)
}

type Login interface {
	Execute(ctx context.Context, in input.Login) (any, error)
}

type MFAVerify interface {
	Execute(ctx context.Context, in input.MFAVerify) (output.Tokens, error)
}

type Refresh interface {
	Execute(ctx context.Context, in input.Refresh) (output.Tokens, error)
}

type Logout interface {
	Execute(ctx context.Context, in input.Logout) error
}

type RecoveryStart interface {
	Execute(ctx context.Context, in input.RecoveryStart) error
}

type RecoveryConfirm interface {
	Execute(ctx context.Context, in input.RecoveryConfirm) error
}

type Me interface {
	Execute(ctx context.Context, userID uuid.UUID) (output.Me, error)
}

type RegisterHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}
type LoginHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}
type MFAVerifyHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}
type RefreshHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}
type LogoutHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}
type RecoveryStartHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}
type RecoveryConfirmHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}
type MeHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}
