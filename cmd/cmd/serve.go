package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/riverqueue/river"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	aggWorker "github.com/AgroBench/backend/internal/aggregation/worker"
	poolWorker "github.com/AgroBench/backend/internal/pool/worker"
	"github.com/AgroBench/backend/internal/validation/worker"
	"github.com/AgroBench/backend/migrations"
	"github.com/AgroBench/backend/pkg/adapter/database"
	"github.com/AgroBench/backend/pkg/adapter/queue"
	"github.com/AgroBench/backend/pkg/adapter/registry"
	"github.com/AgroBench/backend/pkg/adapter/rest"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Sobe a API HTTP (e os workers, em dev)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		db, err := database.Connect(ctx)
		if err != nil {
			return err
		}
		defer db.Close()

		if viper.GetBool("database.auto_migrate") {
			if err := migrations.Up(viper.GetString("database.url")); err != nil {
				return err
			}
			slog.Info("migrations aplicadas")
		}

		adapters, err := registry.Build(db)
		if err != nil {
			return err
		}

		workers := river.NewWorkers()
		river.AddWorker(workers, worker.NewValidate(db, adapters.Enclave, adapters.Chain, adapters.RefData))
		river.AddWorker(workers, aggWorker.NewAggregate(db))
		river.AddWorker(workers, poolWorker.NewDistribute(db, adapters.Chain))

		q, err := queue.Start(ctx, workers)
		if err != nil {
			return err
		}
		defer func() { _ = q.Stop(context.Background()) }()

		return rest.Serve(ctx, db, adapters, q)
	},
}
