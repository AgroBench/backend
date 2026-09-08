package usecase

import (
	"context"

	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/types/input"
)

type Logout struct {
	tokensR contract.RefreshReader
	tokensW contract.RefreshWriter
}

func NewLogout(tokensR contract.RefreshReader, tokensW contract.RefreshWriter) *Logout {
	return &Logout{tokensR: tokensR, tokensW: tokensW}
}

func (u *Logout) Execute(ctx context.Context, in input.Logout) error {
	hash := auth.HashRefresh(in.RefreshToken)
	stored, err := u.tokensR.GetByHash(ctx, hash)
	if err != nil {
		return nil // logout é idempotente
	}
	return u.tokensW.Revoke(ctx, stored.ID)
}
