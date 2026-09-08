package rest

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/httpx"
)

// registerHealth expõe /healthz (processo vivo) e /readyz (banco respondendo).
func registerHealth(r chi.Router, db *sqlx.DB) {
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "database": "down"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "up"})
	})
}
