package cmd

import (
	"errors"
	"gacor/internal/config"
	"gacor/internal/database"
	"gacor/internal/process"
	"gacor/internal/ui"

	"fmt"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show PHP, Nginx, and database status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		phpRunning, phpPid := process.IsRunning("php")
		nginxRunning, nginxPid := process.IsRunning("nginx")

		services := []ui.ServiceStatus{
			{Label: "PHP", Version: cfg.PHPVersion, Running: phpRunning, PID: phpPid},
			{Label: "Nginx", Version: cfg.NginxVersion, Running: nginxRunning, PID: nginxPid},
		}

		if db, dbErr := configuredDatabase(); dbErr == nil {
			running := database.IsRunning(db)
			services = append(services, ui.ServiceStatus{
				Label:   db.Engine.String(),
				Version: db.Version,
				Running: running,
				Detail:  "127.0.0.1:" + itoa(db.Port),
			})
		} else if !errors.Is(dbErr, errNoDatabaseConfigured) {
			return dbErr
		}

		return ui.ShowStatus(services)
	},
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
