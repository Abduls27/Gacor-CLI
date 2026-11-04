package cmd

import (
	"fmt"
	"gacor/internal/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show the current GACOR configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		fmt.Printf("php_version   : %s\n", cfg.PHPVersion)
		fmt.Printf("nginx_version : %s\n", cfg.NginxVersion)
		fmt.Printf("php_workers   : %d\n", cfg.PHPWorkers)
		fmt.Printf("project_folders:\n")
		for _, f := range cfg.ProjectFolders {
			fmt.Printf("  → %s\n", f)
		}
		fmt.Printf("\nTo change the configuration, edit this file: %s\n", config.ConfigPath())
		return nil
	},
}
