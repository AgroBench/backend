package rest

import (
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/cycle/di"
	"github.com/AgroBench/backend/pkg/adapter/queue"
)

func Register(r chi.Router, db *sqlx.DB, q *queue.Queue) {
	h := di.New(db, q)
	r.Get("/cycles", h.List.Handle)
	r.Group(func(adm chi.Router) {
		adm.Use(auth.Require(coredomain.RoleAdmin))
		adm.Post("/admin/cycles", h.Create.Handle)
		adm.Post("/admin/cycles/{id}/close", h.Close.Handle)
	})
}
