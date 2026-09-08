package rest

import (
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/benchmark/handler"
	"github.com/AgroBench/backend/internal/benchmark/usecase"
	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
)

func Register(r chi.Router, db *sqlx.DB) {
	me := handler.NewMe(usecase.NewMe(db))
	rep := handler.NewReport(usecase.NewReport(db))
	r.Group(func(priv chi.Router) {
		priv.Use(auth.Require(coredomain.RoleProducer))
		priv.Get("/benchmark/me", me.Handle)
	})
	r.Group(func(inst chi.Router) {
		inst.Use(auth.Require(coredomain.RoleInstitution))
		inst.Get("/benchmark/report", rep.Handle)
	})
}
