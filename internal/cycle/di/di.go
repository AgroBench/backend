package di

import (
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/cycle/handler"
	"github.com/AgroBench/backend/internal/cycle/repository"
	"github.com/AgroBench/backend/internal/cycle/usecase"
	"github.com/AgroBench/backend/pkg/adapter/queue"
)

type Handlers struct {
	Create *handler.Create
	List   *handler.List
	Close  *handler.Close
}

func New(db *sqlx.DB, q *queue.Queue) *Handlers {
	repo := repository.New(db)
	return &Handlers{
		Create: handler.NewCreate(usecase.NewCreate(repo)),
		List:   handler.NewList(usecase.NewList(repo)),
		Close:  handler.NewClose(usecase.NewClose(repo, q)),
	}
}
