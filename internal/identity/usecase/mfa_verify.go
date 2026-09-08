package usecase

import (
	"context"
	"errors"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/domain"
	"github.com/AgroBench/backend/internal/identity/types/input"
	"github.com/AgroBench/backend/internal/identity/types/output"
)

type MFAVerify struct {
	users   contract.UserReader
	otpsR   contract.OTPReader
	otpsW   contract.OTPWriter
	wallets contract.WalletPubkeyFinder
	refresh contract.RefreshWriter
}

func NewMFAVerify(users contract.UserReader, otpsR contract.OTPReader, otpsW contract.OTPWriter, wallets contract.WalletPubkeyFinder, refresh contract.RefreshWriter) *MFAVerify {
	return &MFAVerify{users: users, otpsR: otpsR, otpsW: otpsW, wallets: wallets, refresh: refresh}
}

func (u *MFAVerify) Execute(ctx context.Context, in input.MFAVerify) (output.Tokens, error) {
	const op = "identity.MFAVerify"
	claims, err := auth.ParseMFA(in.MFAToken)
	if err != nil {
		detail := "mfa_token inválido"
		if errors.Is(err, auth.ErrTokenExpired) {
			detail = "mfa_token expirado"
		}
		return output.Tokens{}, apperrors.Unauthorized(op, err).WithDetail(detail)
	}
	user, err := u.users.GetByID(ctx, claims.UserID())
	if err != nil {
		return output.Tokens{}, apperrors.Unauthorized(op, err).WithDetail("mfa_token inválido")
	}
	if !user.IsActive() {
		return output.Tokens{}, apperrors.Unauthorized(op, errors.New("disabled")).WithDetail("conta desativada")
	}
	if err := ConsumeOTP(ctx, u.otpsR, u.otpsW, user.ID, domain.OTPLogin, in.Code); err != nil {
		return output.Tokens{}, err
	}
	return issueSession(ctx, u.wallets, u.refresh, user, "")
}
