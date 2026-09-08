package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/pool/usecase"
	walletrepo "github.com/AgroBench/backend/internal/wallet/repository"
	"github.com/AgroBench/backend/pkg/adapter/queue"
)

func Register(r chi.Router, db *sqlx.DB, q *queue.Queue) {
	list := usecase.NewListPeriods(db)
	get := usecase.NewGetPeriod(db)
	dist := usecase.NewDistribute(q)
	payouts := usecase.NewListPayouts(db)
	wallets := walletrepo.New(db)

	r.Get("/pool/periods", func(w http.ResponseWriter, r *http.Request) {
		out, err := list.Execute(r.Context())
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	})
	r.Get("/pool/periods/{month}", func(w http.ResponseWriter, r *http.Request) {
		out, err := get.Execute(r.Context(), chi.URLParam(r, "month"))
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	})
	r.Group(func(adm chi.Router) {
		adm.Use(auth.Require(coredomain.RoleAdmin))
		adm.Post("/admin/pool/distribute", func(w http.ResponseWriter, r *http.Request) {
			month := r.URL.Query().Get("month")
			if err := dist.Execute(r.Context(), month); err != nil {
				httpx.Error(w, r, err)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		})
	})
	r.Group(func(priv chi.Router) {
		priv.Use(auth.Require(coredomain.RoleProducer))
		priv.Get("/wallet/payouts", func(w http.ResponseWriter, r *http.Request) {
			wal, err := wallets.GetByUserID(r.Context(), auth.UserID(r.Context()))
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			out, err := payouts.Execute(r.Context(), wal.ID)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			httpx.JSON(w, http.StatusOK, out)
		})
	})
}
