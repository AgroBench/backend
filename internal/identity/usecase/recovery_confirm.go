package usecase

import (
	"context"
	"strings"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/crypto"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/domain"
	"github.com/AgroBench/backend/internal/identity/types/input"
)

type RecoveryConfirm struct {
	users   contract.UserReader
	usersW  contract.UserWriter
	otpsR   contract.OTPReader
	otpsW   contract.OTPWriter
	refresh contract.RefreshWriter
}

func NewRecoveryConfirm(users contract.UserReader, usersW contract.UserWriter, otpsR contract.OTPReader, otpsW contract.OTPWriter, refresh contract.RefreshWriter) *RecoveryConfirm {
	return &RecoveryConfirm{users: users, usersW: usersW, otpsR: otpsR, otpsW: otpsW, refresh: refresh}
}

func (u *RecoveryConfirm) Execute(ctx context.Context, in input.RecoveryConfirm) error {
	const op = "identity.RecoveryConfirm"
	user, err := u.users.GetByCPFHMAC(ctx, auth.HMACIdentifier(in.CPF))
	if err != nil {
		return apperrors.Unauthorized(op, err).WithDetail("código inválido")
	}
	if user.Phone != strings.TrimSpace(in.Phone) {
		return apperrors.Unauthorized(op, err).WithDetail("código inválido")
	}
	if err := ConsumeOTP(ctx, u.otpsR, u.otpsW, user.ID, domain.OTPRecovery, in.Code); err != nil {
		return err
	}
	hash, err := crypto.HashPassword(in.NewPassword)
	if err != nil {
		return apperrors.Internal(op, err)
	}
	if err := u.usersW.UpdatePassword(ctx, user.ID, hash); err != nil {
		return err
	}
	return u.refresh.RevokeAllForUser(ctx, user.ID)
}
