package usecase

import (
	"context"

	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/domain"
	"github.com/AgroBench/backend/internal/identity/types/input"
	"github.com/AgroBench/backend/internal/identity/types/output"
	"github.com/AgroBench/backend/pkg/port"
)

type Login struct {
	users   contract.UserReader
	otps    contract.OTPWriter
	sms     port.SmsSender
	wallets contract.WalletPubkeyFinder
	refresh contract.RefreshWriter
}

func NewLogin(users contract.UserReader, otps contract.OTPWriter, sms port.SmsSender, wallets contract.WalletPubkeyFinder, refresh contract.RefreshWriter) *Login {
	return &Login{users: users, otps: otps, sms: sms, wallets: wallets, refresh: refresh}
}

func (u *Login) Execute(ctx context.Context, in input.Login) (any, error) {
	const op = "identity.Login"
	user, err := u.users.GetByEmail(ctx, normalizeEmail(in.Email))
	if err != nil {
		return nil, invalidCreds(op)
	}
	if !user.IsActive() {
		return nil, invalidCreds(op)
	}
	if err := checkPassword(op, in.Password, user.PasswordHash); err != nil {
		return nil, err
	}
	if user.Role == coredomain.RoleProducer && user.MFAEnabled {
		if err := SendOTP(ctx, u.otps, u.sms, user, domain.OTPLogin); err != nil {
			return nil, err
		}
		tok, err := auth.IssueMFA(user.ID)
		if err != nil {
			return nil, err
		}
		return output.MFAChallenge{MFARequired: true, MFAToken: tok, UserID: user.ID}, nil
	}
	return issueSession(ctx, u.wallets, u.refresh, user, "")
}
