package di

import (
	"github.com/jmoiron/sqlx"

	identityRepo "github.com/AgroBench/backend/internal/identity/repository"
	"github.com/AgroBench/backend/internal/wallet/handler"
	"github.com/AgroBench/backend/internal/wallet/repository"
	"github.com/AgroBench/backend/internal/wallet/usecase"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

type Handlers struct {
	Create *handler.Create
	Get    *handler.Get
	Export *handler.Export
	Repo   *repository.WalletRepo
}

func New(db *sqlx.DB, adapters *registry.Adapters) *Handlers {
	repo := repository.New(db)
	users := identityRepo.NewUserRepo(db)
	otps := identityRepo.NewOTPRepo(db)
	return &Handlers{
		Create: handler.NewCreate(usecase.NewCreate(repo, adapters.Chain)),
		Get:    handler.NewGet(usecase.NewGet(repo, adapters.Chain)),
		Export: handler.NewExport(
			usecase.NewExport(repo, users, otps, otps),
			usecase.NewRequestExportOTP(users, otps, adapters.SMS),
		),
		Repo: repo,
	}
}
