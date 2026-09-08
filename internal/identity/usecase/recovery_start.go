package usecase

import (
	"context"
	"strings"

	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/domain"
	"github.com/AgroBench/backend/internal/identity/types/input"
	"github.com/AgroBench/backend/pkg/port"
)

type RecoveryStart struct {
	users contract.UserReader
	otps  contract.OTPWriter
	sms   port.SmsSender
}

func NewRecoveryStart(users contract.UserReader, otps contract.OTPWriter, sms port.SmsSender) *RecoveryStart {
	return &RecoveryStart{users: users, otps: otps, sms: sms}
}

func (u *RecoveryStart) Execute(ctx context.Context, in input.RecoveryStart) error {
	user, err := u.users.GetByCPFHMAC(ctx, auth.HMACIdentifier(in.CPF))
	if err != nil {
		return nil // não vaza existência
	}
	if user.Phone != strings.TrimSpace(in.Phone) {
		return nil
	}
	if !user.IsActive() {
		return nil
	}
	return SendOTP(ctx, u.otps, u.sms, user, domain.OTPRecovery)
}
