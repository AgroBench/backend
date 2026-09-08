package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/AgroBench/backend/internal/admin/seed"
	"github.com/AgroBench/backend/pkg/adapter/database"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

var seedDemo bool

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Carrega dados base (regiões, culturas, admin). Use --demo para o conjunto da banca.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		db, err := database.Connect(ctx)
		if err != nil {
			return err
		}
		defer db.Close()

		adapters, err := registry.Build(db)
		if err != nil {
			return err
		}

		n, err := seed.Base(ctx, db, adapters)
		if err != nil {
			return fmt.Errorf("seed base: %w", err)
		}
		slog.Info("seed base aplicado", "rows", n)

		if seedDemo {
			if err := seed.Demo(ctx, db, adapters); err != nil {
				return fmt.Errorf("seed demo: %w", err)
			}
			slog.Info("seed demo aplicado")
		}
		return nil
	},
}

func init() {
	seedCmd.Flags().BoolVar(&seedDemo, "demo", false, "inclui produtores, ciclos e instituição da demo")
	rootCmd.AddCommand(seedCmd)
}
