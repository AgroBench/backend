package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/admin/seed"
	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/pkg/adapter/queue"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

func Register(r chi.Router, db *sqlx.DB, adapters *registry.Adapters, q *queue.Queue) {
	r.Group(func(adm chi.Router) {
		adm.Use(auth.Require(coredomain.RoleAdmin))
		adm.Post("/admin/seed/demo", func(w http.ResponseWriter, r *http.Request) {
			if err := seed.Demo(r.Context(), db, adapters); err != nil {
				httpx.Error(w, r, err)
				return
			}
			httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		adm.Post("/admin/jobs/run/{kind}", func(w http.ResponseWriter, r *http.Request) {
			kind := chi.URLParam(r, "kind")
			if q == nil {
				httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"code": "INTERNAL_ERROR", "message": "fila indisponível"})
				return
			}
			var err error
			switch kind {
			case "distribute_pool":
				err = q.EnqueueDistribute(r.Context(), r.URL.Query().Get("month"))
			default:
				httpx.JSON(w, http.StatusBadRequest, map[string]string{"code": "INVALID_INPUT", "message": "job desconhecido"})
				return
			}
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		})
	})
}
