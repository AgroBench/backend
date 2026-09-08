package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/AgroBench/backend/internal/core/config"
)

var configFile string

var rootCmd = &cobra.Command{
	Use:   "agrobench",
	Short: "AgroBench API — benchmarking regional com dados anonimizados",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Load(configFile); err != nil {
			return fmt.Errorf("carregando config: %w", err)
		}
		config.SetupLogger()
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "caminho do config.json (default: $CONFIG_FILE ou config/env/config.dev.json)")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
}
