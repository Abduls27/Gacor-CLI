package cmd

import (
	"gacor/internal/config"
	"gacor/internal/nginx"
	"gacor/internal/project"
	"gacor/internal/ssl"
	"gacor/internal/ui"
	"path/filepath"

	"github.com/spf13/cobra"
)

var secureCmd = &cobra.Command{
	Use:   "secure",
	Short: "Set up HTTPS for all projects with mkcert",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Banner()

		// 1. Download mkcert if it is not installed
		if !ssl.IsInstalled() {
			ui.Info("Downloading mkcert...")
			if err := ssl.Download(); err != nil {
				ui.Error(err.Error())
				return nil
			}
			ui.Success("mkcert downloaded")
		}

		// 2. Install root CA
		ui.Info("Installing root CA...")
		if err := ssl.InstallCA(); err != nil {
			ui.Error("Failed to install the CA: " + err.Error())
			return nil
		}
		ui.Success("Root CA installed")

		// 3. Generate a certificate for each domain
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		projects := project.ListProjects(cfg)
		domains := nginx.DomainsFromProjects(projects)

		ui.Info("Generating certificates...")
		if err := ssl.GenerateCert(domains); err != nil {
			ui.Error(err.Error())
			return nil
		}

		// 4. Regenerate nginx.conf with HTTPS
		nginxDir := filepath.Dir(nginx.ExePath(cfg.NginxVersion))
		confPath := filepath.Join(nginxDir, "conf", "nginx.conf")
		ui.Info("Updating nginx.conf with HTTPS...")
		if err := nginx.GenerateConfSSL(projects, confPath); err != nil {
			ui.Error(err.Error())
			return nil
		}

		ui.Success("HTTPS is ready! Run: gacor stop && gacor run")
		return nil
	},
}
