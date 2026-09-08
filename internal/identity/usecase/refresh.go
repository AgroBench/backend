package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/types/input"
	"github.com/AgroBench/backend/internal/identity/types/output"
)

type Refresh struct {
	users   contract.UserReader
	tokensR contract.RefreshReader
	tokensW contract.RefreshWriter
	wallets contract.WalletPubkeyFinder
}

func NewRefresh(users contract.UserReader, tokensR contract.RefreshReader, tokensW contract.RefreshWriter, wallets contract.WalletPubkeyFinder) *Refresh {
	return &Refresh{users: users, tokensR: tokensR, tokensW: tokensW, wallets: wallets}
}

func (u *Refresh) Execute(ctx context.Context, in input.Refresh) (output.Tokens, error) {
	const op = "identity.Refresh"
	hash := auth.HashRefresh(in.RefreshToken)
	stored, err := u.tokensR.GetByHash(ctx, hash)
	if err != nil {
		return output.Tokens{}, apperrors.Unauthorized(op, err).WithDetail("refresh token inválido")
	}
	if stored.Revoked() {
		// Reuso detectado: revoga todos os tokens do usuário (README §10).
		_ = u.tokensW.RevokeAllForUser(ctx, stored.UserID)
		return output.Tokens{}, apperrors.Unauthorized(op, errors.New("reuse")).WithDetail("refresh token inválido")
	}
	if stored.Expired(time.Now()) {
		_ = u.tokensW.Revoke(ctx, stored.ID)
		return output.Tokens{}, apperrors.Unauthorized(op, errors.New("expired")).WithDetail("refresh token expirado")
	}
	user, err := u.users.GetByID(ctx, stored.UserID)
	if err != nil || !user.IsActive() {
		return output.Tokens{}, apperrors.Unauthorized(op, err).WithDetail("refresh token inválido")
	}
	if err := u.tokensW.Revoke(ctx, stored.ID); err != nil {
		return output.Tokens{}, err
	}
	device := in.Device
	if device == "" {
		device = stored.Device
	}
	return issueSession(ctx, u.wallets, u.tokensW, user, device)
}
