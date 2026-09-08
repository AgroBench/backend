package contract

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/domain"
	walletdomain "github.com/AgroBench/backend/internal/wallet/domain"
	"github.com/AgroBench/backend/internal/wallet/types/input"
	"github.com/AgroBench/backend/internal/wallet/types/output"
)

type WalletRepo interface {
	Create(ctx context.Context, w walletdomain.Wallet) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (walletdomain.Wallet, error)
	GetByID(ctx context.Context, id uuid.UUID) (walletdomain.Wallet, error)
	MarkExported(ctx context.Context, id uuid.UUID) error
	SumRewards(ctx context.Context, walletID uuid.UUID) (domain.MicroUSDC, error)
}

type Create interface {
	Execute(ctx context.Context, userID uuid.UUID, in input.CreateWallet) (output.Wallet, error)
}

type Get interface {
	Execute(ctx context.Context, userID uuid.UUID) (output.Wallet, error)
}

type Export interface {
	Execute(ctx context.Context, userID uuid.UUID, code string) (output.Export, error)
}

type RequestExportOTP interface {
	Execute(ctx context.Context, userID uuid.UUID) error
}

type CreateHandler interface {
	Handle(http.ResponseWriter, *http.Request)
}
type GetHandler interface {
	Handle(http.ResponseWriter, *http.Request)
}
type ExportHandler interface {
	Handle(http.ResponseWriter, *http.Request)
}
