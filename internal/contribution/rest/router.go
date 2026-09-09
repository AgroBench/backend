package rest

import (
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/contribution/di"
	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/adapter/queue"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

func Register(r chi.Router, db *sqlx.DB, adapters *registry.Adapters, q *queue.Queue) {
	h := di.New(db, adapters, q)
	r.Get("/enclave/public-key", h.EnclaveKey.Handle)
	r.Group(func(priv chi.Router) {
		priv.Use(auth.Require(coredomain.RoleProducer))
		priv.Post("/chain/stake/lock-tx", h.LockTx.Handle)
		priv.Post("/chain/stake/lock-submit", h.LockSubmit.Handle)
		priv.Post("/contributions/commit", h.Commit.Handle)
		priv.Post("/contributions/{id}/reveal", h.Reveal.Handle)
		priv.Post("/contributions/{id}/release-stake/tx", h.ReleaseTx.Handle)
		priv.Post("/contributions/{id}/release-stake/submit", h.ReleaseSubmit.Handle)
		priv.Get("/contributions", h.List.Handle)
		priv.Get("/contributions/{id}", h.Get.Handle)
	})
}
