package usecase

import (
	"context"
	"encoding/base64"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/wallet/contract"
	"github.com/AgroBench/backend/internal/wallet/domain"
	"github.com/AgroBench/backend/internal/wallet/types/input"
	"github.com/AgroBench/backend/internal/wallet/types/output"
	"github.com/AgroBench/backend/pkg/port"
)

type Create struct {
	repo  contract.WalletRepo
	chain port.ChainClient
}

func NewCreate(repo contract.WalletRepo, chain port.ChainClient) *Create {
	return &Create{repo: repo, chain: chain}
}

func (u *Create) Execute(ctx context.Context, userID uuid.UUID, in input.CreateWallet) (output.Wallet, error) {
	const op = "wallet.Create"
	blob, err := base64.StdEncoding.DecodeString(in.EncryptedBlob)
	if err != nil {
		return output.Wallet{}, apperrors.Validation(op, err).WithDetail("encrypted_blob deve ser base64")
	}
	if in.BlobVersion < 1 {
		in.BlobVersion = 1
	}
	w := domain.Wallet{
		ID:            coredomain.NewID(),
		UserID:        userID,
		Pubkey:        in.Pubkey,
		EncryptedBlob: blob,
		BlobVersion:   in.BlobVersion,
	}
	if err := u.repo.Create(ctx, w); err != nil {
		if !apperrors.Is(err, apperrors.ErrConflict) {
			return output.Wallet{}, err
		}
		claimed, claimErr := u.repo.ClaimPlaceholder(ctx, userID, in.Pubkey, blob, in.BlobVersion)
		if claimErr != nil {
			if apperrors.Is(claimErr, apperrors.ErrNotFound) {
				return output.Wallet{}, apperrors.Conflict(op, err).WithDetail("wallet já cadastrada")
			}
			return output.Wallet{}, claimErr
		}
		return u.toOutput(ctx, claimed)
	}
	return u.toOutput(ctx, w)
}

type Get struct {
	repo  contract.WalletRepo
	chain port.ChainClient
}

func NewGet(repo contract.WalletRepo, chain port.ChainClient) *Get {
	return &Get{repo: repo, chain: chain}
}

func (u *Get) Execute(ctx context.Context, userID uuid.UUID) (output.Wallet, error) {
	w, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		return output.Wallet{}, err
	}
	return u.toOutput(ctx, w)
}

func (u *Get) toOutput(ctx context.Context, w domain.Wallet) (output.Wallet, error) {
	return compose(ctx, u.repo, u.chain, w)
}

func (u *Create) toOutput(ctx context.Context, w domain.Wallet) (output.Wallet, error) {
	return compose(ctx, u.repo, u.chain, w)
}

func compose(ctx context.Context, repo contract.WalletRepo, chain port.ChainClient, w domain.Wallet) (output.Wallet, error) {
	var bal coredomain.MicroUSDC
	if chain != nil {
		b, err := chain.Balance(ctx, port.Account(w.Pubkey))
		if err != nil {
			return output.Wallet{}, apperrors.External("wallet.Balance", 0, err)
		}
		bal = b
	}
	reward, _ := repo.SumRewards(ctx, w.ID)
	out := output.NewWallet(w.ID, w.Pubkey, w.BlobVersion, bal, reward)
	if w.ExportedAt != nil {
		s := w.ExportedAt.UTC().Format("2006-01-02T15:04:05Z")
		out.ExportedAt = &s
	}
	return out, nil
}
