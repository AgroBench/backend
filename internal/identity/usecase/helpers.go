package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/crypto"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/domain"
	"github.com/AgroBench/backend/internal/identity/types/output"
	"github.com/AgroBench/backend/pkg/port"
)

func issueSession(ctx context.Context, wallets contract.WalletPubkeyFinder, refresh contract.RefreshWriter, user domain.User, device string) (output.Tokens, error) {
	const op = "identity.issueSession"
	wallet := ""
	if wallets != nil {
		pk, err := wallets.GetPubkeyByUserID(ctx, user.ID)
		if err != nil && !apperrors.Is(err, apperrors.ErrNotFound) {
			return output.Tokens{}, err
		}
		wallet = pk
	}
	access, err := auth.IssueAccess(user.ID, user.Role, wallet)
	if err != nil {
		return output.Tokens{}, apperrors.Internal(op, err)
	}
	plain, hash, err := auth.NewRefreshToken()
	if err != nil {
		return output.Tokens{}, apperrors.Internal(op, err)
	}
	tok := domain.RefreshToken{
		ID:        coredomain.NewID(),
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(auth.RefreshTTL()),
		Device:    device,
	}
	if err := refresh.Create(ctx, tok); err != nil {
		return output.Tokens{}, err
	}
	return output.NewTokens(access, plain), nil
}

func SendOTP(ctx context.Context, otpsW contract.OTPWriter, sms port.SmsSender, user domain.User, purpose domain.OTPPurpose) error {
	const op = "identity.sendOTP"
	if err := otpsW.InvalidatePending(ctx, user.ID, purpose); err != nil {
		return err
	}
	plain, hash, err := auth.GenerateOTP()
	if err != nil {
		return apperrors.Internal(op, err)
	}
	o := domain.OTPCode{
		ID:        coredomain.NewID(),
		UserID:    user.ID,
		Purpose:   purpose,
		CodeHash:  hash,
		ExpiresAt: time.Now().Add(auth.OTPTL()),
	}
	if err := otpsW.Create(ctx, o); err != nil {
		return err
	}
	if err := sms.Send(ctx, user.Phone, auth.OTPMessage(plain)); err != nil {
		return apperrors.External(op, 0, err).WithDetail("falha ao enviar SMS")
	}
	return nil
}

func ConsumeOTP(ctx context.Context, otpsR contract.OTPReader, otpsW contract.OTPWriter, userID uuid.UUID, purpose domain.OTPPurpose, code string) error {
	const op = "identity.verifyOTP"
	o, err := otpsR.GetPending(ctx, userID, purpose)
	if err != nil {
		return apperrors.Unauthorized(op, err).WithDetail("código inválido")
	}
	now := time.Now()
	if o.Expired(now) {
		_ = otpsW.Consume(ctx, o.ID)
		return apperrors.Unauthorized(op, errors.New("expired")).WithDetail("código expirado")
	}
	if o.Attempts >= auth.OTPMaxAttempts() {
		_ = otpsW.Consume(ctx, o.ID)
		return apperrors.Unauthorized(op, errors.New("max attempts")).WithDetail("código inválido")
	}
	if !auth.OTPMatch(code, o.CodeHash) {
		_ = otpsW.IncrementAttempts(ctx, o.ID)
		return apperrors.Unauthorized(op, errors.New("mismatch")).WithDetail("código inválido")
	}
	return otpsW.Consume(ctx, o.ID)
}

func invalidCreds(op string) error {
	return apperrors.Unauthorized(op, errors.New("invalid credentials")).WithDetail("credenciais inválidas")
}

func checkPassword(op, password, hash string) error {
	if err := crypto.VerifyPassword(password, hash); err != nil {
		return invalidCreds(op)
	}
	return nil
}

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
