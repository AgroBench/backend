package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/riverqueue/river"
	"github.com/spf13/cobra"

	aggWorker "github.com/AgroBench/backend/internal/aggregation/worker"
	poolWorker "github.com/AgroBench/backend/internal/pool/worker"
	"github.com/AgroBench/backend/internal/validation/worker"
	"github.com/AgroBench/backend/pkg/adapter/database"
	"github.com/AgroBench/backend/pkg/adapter/queue"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Sobe só os workers River (produção)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		db, err := database.Connect(ctx)
		if err != nil {
			return err
		}
		defer db.Close()

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
		<-ctx.Done()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)
}
