// Package rest monta o router chi, os middlewares globais e registra os routers de cada módulo.
package rest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"

	adminRest "github.com/AgroBench/backend/internal/admin/rest"
	benchmarkRest "github.com/AgroBench/backend/internal/benchmark/rest"
	carRest "github.com/AgroBench/backend/internal/car/rest"
	contributionRest "github.com/AgroBench/backend/internal/contribution/rest"
	"github.com/AgroBench/backend/internal/core/httpx"
	cycleRest "github.com/AgroBench/backend/internal/cycle/rest"
	identityRest "github.com/AgroBench/backend/internal/identity/rest"
	institutionRest "github.com/AgroBench/backend/internal/institution/rest"
	poolRest "github.com/AgroBench/backend/internal/pool/rest"
	walletrepo "github.com/AgroBench/backend/internal/wallet/repository"
	walletRest "github.com/AgroBench/backend/internal/wallet/rest"
	"github.com/AgroBench/backend/pkg/adapter/queue"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

func NewRouter(db *sqlx.DB, adapters *registry.Adapters, q *queue.Queue) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(httpx.RequestLogger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(middleware.Throttle(200))
	r.Use(httprate.LimitByIP(120, time.Minute))
	r.Use(middleware.RequestSize(2 << 20))
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(httpx.ContentTypeJSON)

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND", "message": "Rota não encontrada"})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusMethodNotAllowed, map[string]string{"code": "METHOD_NOT_ALLOWED", "message": "Método não permitido"})
	})

	registerHealth(r, db)

	r.Route("/api/v1", func(api chi.Router) {
		wallets := walletrepo.New(db)
		identityRest.Register(api, db, adapters, wallets)
		walletRest.Register(api, db, adapters)
		carRest.Register(api, db, adapters)
		cycleRest.Register(api, db, q)
		contributionRest.Register(api, db, adapters, q)
		benchmarkRest.Register(api, db)
		institutionRest.Register(api, db, adapters)
		poolRest.Register(api, db, q)
		adminRest.Register(api, db, adapters, q)
	})

	return r
}

// Serve sobe o servidor HTTP e faz graceful shutdown quando ctx é cancelado (SIGINT/SIGTERM).
func Serve(ctx context.Context, db *sqlx.DB, adapters *registry.Adapters, q *queue.Queue) error {
	port := viper.GetString("server.http.port")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      NewRouter(db, adapters, q),
		ReadTimeout:  time.Duration(viper.GetInt("server.http.read_timeout_seconds")) * time.Second,
		WriteTimeout: time.Duration(viper.GetInt("server.http.write_timeout_seconds")) * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server iniciado", "port", port, "env", viper.GetString("app.env"))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	slog.Info("encerrando http server")
	timeout := time.Duration(viper.GetInt("server.http.shutdown_timeout_seconds")) * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
