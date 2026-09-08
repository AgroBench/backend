package di

import (
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/car/handler"
	"github.com/AgroBench/backend/internal/car/repository"
	"github.com/AgroBench/backend/internal/car/usecase"
	walletrepo "github.com/AgroBench/backend/internal/wallet/repository"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

type Handlers struct {
	Create       *handler.CreateProperty
	Get          *handler.GetProperty
	ListRegions  *handler.ListRegions
	ListCultures *handler.ListCultures
}

func New(db *sqlx.DB, adapters *registry.Adapters) *Handlers {
	props := repository.NewProperty(db)
	catalog := repository.NewCatalog(db)
	wallets := walletrepo.New(db)
	return &Handlers{
		Create:       handler.NewCreateProperty(usecase.NewCreateProperty(props, catalog, adapters.Sicar, adapters.Chain, wallets)),
		Get:          handler.NewGetProperty(usecase.NewGetProperty(props)),
		ListRegions:  handler.NewListRegions(usecase.NewListRegions(catalog)),
		ListCultures: handler.NewListCultures(usecase.NewListCultures(catalog)),
	}
}
