package di

import (
	"github.com/jmoiron/sqlx"

	carrepo "github.com/AgroBench/backend/internal/car/repository"
	"github.com/AgroBench/backend/internal/contribution/handler"
	"github.com/AgroBench/backend/internal/contribution/repository"
	"github.com/AgroBench/backend/internal/contribution/usecase"
	cyclerepo "github.com/AgroBench/backend/internal/cycle/repository"
	walletrepo "github.com/AgroBench/backend/internal/wallet/repository"
	"github.com/AgroBench/backend/pkg/adapter/queue"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

type Handlers struct {
	Commit     *handler.Commit
	Reveal     *handler.Reveal
	List       *handler.List
	Get        *handler.Get
	EnclaveKey *handler.EnclaveKey
}

func New(db *sqlx.DB, adapters *registry.Adapters, q *queue.Queue) *Handlers {
	repo := repository.New(db)
	cycles := cyclerepo.New(db)
	props := carrepo.NewProperty(db)
	wallets := walletrepo.New(db)
	return &Handlers{
		Commit:     handler.NewCommit(usecase.NewCommit(repo, cycles, props, wallets, adapters.Chain)),
		Reveal:     handler.NewReveal(usecase.NewReveal(repo, q)),
		List:       handler.NewList(usecase.NewList(repo), wallets),
		Get:        handler.NewGet(usecase.NewGet(repo)),
		EnclaveKey: handler.NewEnclaveKey(usecase.NewEnclaveKey(adapters.Enclave)),
	}
}
