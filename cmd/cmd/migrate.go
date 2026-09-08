package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/AgroBench/backend/migrations"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate [up|down|version]",
	Short: "Aplica ou reverte migrations embutidas no binário",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := viper.GetString("database.url")
		switch args[0] {
		case "up":
			return migrations.Up(url)
		case "down":
			return migrations.Down(url)
		case "version":
			v, dirty, err := migrations.Version(url)
			if err != nil {
				return err
			}
			fmt.Printf("version=%d dirty=%v\n", v, dirty)
			return nil
		default:
			return fmt.Errorf("ação desconhecida: %s (use up|down|version)", args[0])
		}
	},
}
