package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/AgroBench/backend/migrations"
	"github.com/AgroBench/backend/pkg/adapter/database"
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

		return rest.Serve(ctx, db, adapters)
	},
}
