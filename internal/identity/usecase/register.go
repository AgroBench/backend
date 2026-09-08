package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/crypto"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/domain"
	"github.com/AgroBench/backend/internal/identity/types/input"
	"github.com/AgroBench/backend/internal/identity/types/output"
	"github.com/AgroBench/backend/pkg/port"
)

type Register struct {
	users contract.UserWriter
	otps  contract.OTPWriter
	sms   port.SmsSender
}

func NewRegister(users contract.UserWriter, otps contract.OTPWriter, sms port.SmsSender) *Register {
	return &Register{users: users, otps: otps, sms: sms}
}

func (u *Register) Execute(ctx context.Context, in input.Register) (output.MFAChallenge, error) {
	const op = "identity.Register"
	if len(crypto.NormalizeIdentifier(in.CPF)) != 11 {
		return output.MFAChallenge{}, apperrors.Validation(op, errors.New("cpf")).WithDetail("CPF inválido")
	}
	hash, err := crypto.HashPassword(in.Password)
	if err != nil {
		return output.MFAChallenge{}, apperrors.Internal(op, err)
	}
	user := domain.User{
		ID:           coredomain.NewID(),
		Email:        normalizeEmail(in.Email),
		Phone:        strings.TrimSpace(in.Phone),
		CPFHMAC:      auth.HMACIdentifier(in.CPF),
		PasswordHash: hash,
		Role:         coredomain.RoleProducer,
		MFAEnabled:   true,
		Status:       domain.UserActive,
	}
	if err := u.users.Create(ctx, user); err != nil {
		if apperrors.Is(err, apperrors.ErrConflict) {
			return output.MFAChallenge{}, apperrors.Conflict(op, err).WithDetail("email ou CPF já cadastrado")
		}
		return output.MFAChallenge{}, err
	}
	if err := SendOTP(ctx, u.otps, u.sms, user, domain.OTPLogin); err != nil {
		return output.MFAChallenge{}, err
	}
	tok, err := auth.IssueMFA(user.ID)
	if err != nil {
		return output.MFAChallenge{}, apperrors.Internal(op, err)
	}
	return output.MFAChallenge{MFARequired: true, MFAToken: tok, UserID: user.ID}, nil
}
