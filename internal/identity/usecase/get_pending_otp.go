package usecase

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/types/output"
	smsmock "github.com/AgroBench/backend/pkg/adapter/sms/mock"
)

var otpRe = regexp.MustCompile(`\b(\d{6})\b`)

type GetPendingOTP struct {
	users contract.UserReader
	sms   *smsmock.Sender
}

func NewGetPendingOTP(users contract.UserReader, sms *smsmock.Sender) *GetPendingOTP {
	return &GetPendingOTP{users: users, sms: sms}
}

func (u *GetPendingOTP) Execute(ctx context.Context, userID uuid.UUID) (output.PendingOTP, error) {
	const op = "identity.GetPendingOTP"
	if u.sms == nil {
		return output.PendingOTP{}, apperrors.NotFound(op, errors.New("sms not mock")).WithDetail("endpoint só existe com adapters.sms=mock")
	}
	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return output.PendingOTP{}, err
	}
	msg, ok := u.sms.LastTo(user.Phone)
	if !ok {
		return output.PendingOTP{}, apperrors.NotFound(op, errors.New("no otp")).WithDetail("nenhum OTP pendente")
	}
	code := otpRe.FindString(msg.Body)
	purpose := "login"
	if strings.Contains(strings.ToLower(msg.Body), "recupera") {
		purpose = "recovery"
	}
	return output.PendingOTP{UserID: userID, Code: code, Purpose: purpose}, nil
}
