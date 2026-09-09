package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/AgroBench/backend/pkg/adapter/database"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

var chainInitCmd = &cobra.Command{
	Use:   "chain-init",
	Short: "Chama initialize do programa Anchor agrobench (uma vez na Devnet)",
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
		if adapters.Chain.ProgramID() == "" {
			return fmt.Errorf("CHAIN_SOLANA_PROGRAM_ID vazio — nada a inicializar")
		}
		ref, err := adapters.Chain.InitializeProgram(ctx)
		if err != nil {
			return fmt.Errorf("initialize: %w", err)
		}
		slog.Info("programa inicializado", "signature", ref, "program_id", adapters.Chain.ProgramID(), "pool", adapters.Chain.Pool())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(chainInitCmd)
}
