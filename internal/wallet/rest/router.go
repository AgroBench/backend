package rest

import (
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/wallet/di"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

func Register(r chi.Router, db *sqlx.DB, adapters *registry.Adapters) *di.Handlers {
	h := di.New(db, adapters)
	r.Group(func(priv chi.Router) {
		priv.Use(auth.Require(coredomain.RoleProducer))
		priv.Post("/wallet", h.Create.Handle)
		priv.Get("/wallet", h.Get.Handle)
		priv.Get("/wallet/export", h.Export.Handle)
	})
	return h
}
