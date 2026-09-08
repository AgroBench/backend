package usecase

import (
	"context"
	"encoding/base64"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/identity/contract"
	iddomain "github.com/AgroBench/backend/internal/identity/domain"
	identityUC "github.com/AgroBench/backend/internal/identity/usecase"
	wcontract "github.com/AgroBench/backend/internal/wallet/contract"
	"github.com/AgroBench/backend/internal/wallet/types/output"
	"github.com/AgroBench/backend/pkg/port"
)

// Export devolve o blob cifrado após OTP (purpose login). README §7.
type Export struct {
	repo  wcontract.WalletRepo
	users contract.UserReader
	otpsR contract.OTPReader
	otpsW contract.OTPWriter
}

func NewExport(repo wcontract.WalletRepo, users contract.UserReader, otpsR contract.OTPReader, otpsW contract.OTPWriter) *Export {
	return &Export{repo: repo, users: users, otpsR: otpsR, otpsW: otpsW}
}

func (u *Export) Execute(ctx context.Context, userID uuid.UUID, code string) (output.Export, error) {
	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return output.Export{}, err
	}
	if err := identityUC.ConsumeOTP(ctx, u.otpsR, u.otpsW, user.ID, iddomain.OTPLogin, code); err != nil {
		return output.Export{}, err
	}
	w, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		return output.Export{}, err
	}
	_ = u.repo.MarkExported(ctx, w.ID)
	return output.Export{
		Pubkey:        w.Pubkey,
		EncryptedBlob: base64.StdEncoding.EncodeToString(w.EncryptedBlob),
		BlobVersion:   w.BlobVersion,
	}, nil
}

type RequestExportOTP struct {
	users contract.UserReader
	otps  contract.OTPWriter
	sms   port.SmsSender
}

func NewRequestExportOTP(users contract.UserReader, otps contract.OTPWriter, sms port.SmsSender) *RequestExportOTP {
	return &RequestExportOTP{users: users, otps: otps, sms: sms}
}

func (u *RequestExportOTP) Execute(ctx context.Context, userID uuid.UUID) error {
	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	return identityUC.SendOTP(ctx, u.otps, u.sms, user, iddomain.OTPLogin)
}
