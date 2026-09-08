package rest

import (
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/car/di"
	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

func Register(r chi.Router, db *sqlx.DB, adapters *registry.Adapters) {
	h := di.New(db, adapters)
	r.Get("/micro-regions", h.ListRegions.Handle)
	r.Get("/cultures", h.ListCultures.Handle)
	r.Group(func(priv chi.Router) {
		priv.Use(auth.Require(coredomain.RoleProducer))
		priv.Post("/property", h.Create.Handle)
		priv.Get("/property", h.Get.Handle)
	})
}
