package cmd

import (
	"errors"
	"time"

	"fmt"
	"gacor/internal/config"
	"gacor/internal/database"
	"gacor/internal/hosts"
	"gacor/internal/nginx"
	"gacor/internal/process"
	"gacor/internal/project"
	"gacor/internal/ssl"
	"gacor/internal/ui"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var elevated bool

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start PHP, Nginx, and the selected database in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()

		ui.StepTimed("Welcome to Gacor - starting your Dev Environment")
		ui.StepTimed("Spinning up PHP, Nginx, and database...")

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if cfg.PHPVersion == "" {
			return errors.New("PHP version has not been selected. Run: gacor php use")
		}
		if cfg.NginxVersion == "" {
			return errors.New("Nginx version has not been selected. Run: gacor nginx use")
		}

		if db, dbErr := configuredDatabase(); dbErr == nil {
			if !database.IsRunning(db) {
				ui.StepTimed(fmt.Sprintf("Starting %s %s...", db.Engine.String(), db.Version))
				if err := database.Start(db); err != nil {
					return err
				}
				ui.StepDetail(fmt.Sprintf("Running on port %d", db.Port))
			}
		} else if !errors.Is(dbErr, errNoDatabaseConfigured) {
			return dbErr
		}

		// 1. Detect projects
		projects := project.ListProjects(cfg)
		if len(projects) == 0 {
			ui.Warn("No Laravel projects detected.")
			ui.Info("Add a folder with: gacor project add <path>")
		}

		// 2. Automatically generate nginx.conf
		nginxDir := filepath.Dir(nginx.ExePath(cfg.NginxVersion))
		confPath := filepath.Join(nginxDir, "conf", "nginx.conf")
		ui.StepTimed("Generating nginx.conf...")

		firstProject := ""
		if len(projects) > 0 {
			firstProject = strings.ToLower(filepath.Base(projects[0])) + ".test"
		}

		if firstProject != "" && ssl.CertExists(firstProject) {
			if err := nginx.GenerateConfSSL(projects, confPath); err != nil {
				return fmt.Errorf("gagal generate nginx.conf: %w", err)
			}
		} else {
			if err := nginx.GenerateConf(projects, confPath); err != nil {
				return fmt.Errorf("gagal generate nginx.conf: %w", err)
			}
		}
		ui.StepDetail(fmt.Sprintf("%d project(s) configured", len(projects)))

		// 3. Update hosts if needed
		domains := nginx.DomainsFromProjects(projects)
		if hosts.NeedsUpdate(domains) {
			ui.StepTimed("Hosts file needs updating; requesting administrator access...")
			if err := process.RelaunchAsAdminWithArgs("hosts-sync"); err != nil {
				return fmt.Errorf("failed to request administrator access to update the hosts file: %w", err)
			}
			for i := 0; i < 30 && hosts.NeedsUpdate(domains); i++ {
				time.Sleep(500 * time.Millisecond)
			}
			if hosts.NeedsUpdate(domains) {
				return fmt.Errorf("hosts file was not updated; try running gacor run again")
			}
			ui.StepDetail("Hosts file updated")
		} else {
			ui.StepTimed("Hosts file is up to date")
		}

		// 4. Start PHP-FPM
		ui.StepTimed(fmt.Sprintf("Starting PHP %s...", cfg.PHPVersion))
		if err := startPHPService(cfg); err != nil {
			return err
		}
		ui.StepDetail("Running on port 9000")

		// 5. Start Nginx
		nginxExe := nginx.ExePath(cfg.NginxVersion)
		ui.StepTimed(fmt.Sprintf("Starting Nginx %s...", cfg.NginxVersion))
		if err := process.StartBackground("nginx", nginxExe, []string{}, nginxDir, nil); err != nil {
			if stopErr := process.Stop("php"); stopErr != nil {
				return errors.Join(err, fmt.Errorf("failed to stop PHP after Nginx startup failed: %w", stopErr))
			}
			return err
		}
		ui.StepDetail("Running on port 80")

		ui.StepTimed("All services are running. Use 'gacor status' to see details.")
		fmt.Println()

		return nil
	},
}

func init() {
	runCmd.Flags().BoolVar(&elevated, "elevated", false, "internal flag: already running as administrator")
}
