package cmd

import (
	"fmt"
	"gacor/internal/config"
	"gacor/internal/nginx"
	"gacor/internal/process"
	"gacor/internal/ui"

	"github.com/spf13/cobra"
)

var nginxCmd = &cobra.Command{
	Use:   "nginx",
	Short: "Manage Nginx versions",
}

var nginxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available Nginx versions (from nginx.org)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Info("Fetching Nginx versions from nginx.org...")
		versions, err := nginx.FetchAvailableVersions()
		if err != nil {
			ui.Error(err.Error())
			return nil
		}
		cfg, _ := config.Load()
		for _, v := range nginx.SortedVersions(versions) {
			installed := ""
			active := ""
			if nginx.IsInstalled(v) {
				installed = " [installed]"
			}
			if cfg.NginxVersion == v {
				active = " ◀ active"
			}
			fmt.Printf("  %s%s%s\n", v, installed, active)
		}
		return nil
	},
}

var nginxUseCmd = &cobra.Command{
	Use:   "use",
	Short: "Select an Nginx version using the arrow keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Info("Fetching Nginx versions from nginx.org...")
		versions, err := nginx.FetchAvailableVersions()
		if err != nil {
			ui.Error(err.Error())
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		items := nginx.SortedVersions(versions)
		labels := make([]string, len(items))
		for i, v := range items {
			labels[i] = v
			if nginx.IsInstalled(v) {
				labels[i] += " (installed)"
			}
			if cfg.NginxVersion == v {
				labels[i] += " ◀ active"
			}
		}

		idx, err := ui.SelectIndexed("Select an Nginx version:", labels)
		if err != nil {
			return err
		}
		if idx < 0 {
			ui.Warn("Cancelled")
			return nil
		}
		chosen := items[idx]

		if !nginx.IsInstalled(chosen) {
			err := ui.RunWithProgress(func(onEvent func(ui.ProgressEvent)) error {
				return nginx.DownloadFromURL(chosen, versions[chosen], func(stage, label string, done, total int64) {
					onEvent(ui.ProgressEvent{Stage: stage, Label: label, Done: done, Total: total})
				})
			})
			if err != nil {
				return err
			}
		} else {
			ui.Info(fmt.Sprintf("Nginx %s is already installed; skipping download", chosen))
		}

		cfg.NginxVersion = chosen
		if err := cfg.Save(); err != nil {
			return err
		}
		ui.Success(fmt.Sprintf("Nginx %s set as the active version", chosen))
		return nil
	},
}

var nginxInstallCmd = &cobra.Command{
	Use:   "install [version]",
	Short: "Install a specific Nginx version",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Info("Fetching download URLs from nginx.org...")
		versions, err := nginx.FetchAvailableVersions()
		if err != nil {
			return err
		}
		version := ""
		if len(args) == 1 {
			version = args[0]
		} else {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			items := nginx.SortedVersions(versions)
			labels := make([]string, len(items))
			for i, v := range items {
				labels[i] = v
				if nginx.IsInstalled(v) {
					labels[i] += " (installed)"
				}
				if cfg.NginxVersion == v {
					labels[i] += " ◀ active"
				}
			}
			idx, err := ui.SelectIndexed("Select an Nginx version:", labels)
			if err != nil {
				return err
			}
			if idx < 0 {
				ui.Warn("Cancelled")
				return nil
			}
			version = items[idx]
		}
		if nginx.IsInstalled(version) {
			ui.Info(fmt.Sprintf("Nginx %s is already installed", version))
			return nil
		}
		url, ok := versions[version]
		if !ok {
			return fmt.Errorf("Nginx version %s was not found", version)
		}

		return ui.RunWithProgress(func(onEvent func(ui.ProgressEvent)) error {
			return nginx.DownloadFromURL(version, url, func(stage, label string, done, total int64) {
				onEvent(ui.ProgressEvent{Stage: stage, Label: label, Done: done, Total: total})
			})
		})
	},
}

var nginxCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for and install the latest Nginx version",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.NginxVersion == "" {
			ui.Warn("No active Nginx version is configured")
			return nil
		}

		ui.Info("Checking for updates...")
		versions, err := nginx.FetchAvailableVersions()
		if err != nil {
			ui.Error(err.Error())
			return nil
		}

		sorted := nginx.SortedVersions(versions)
		if len(sorted) == 0 {
			ui.Warn("No Nginx versions were found on nginx.org")
			return nil
		}
		latest := sorted[0]

		if latest == cfg.NginxVersion {
			ui.Success(fmt.Sprintf("Nginx %s is already up to date", cfg.NginxVersion))
			return nil
		}

		label := fmt.Sprintf("Update Nginx from %s to %s?", cfg.NginxVersion, latest)
		idx, err := ui.SelectIndexedWithSearch(label, []string{"Yes, update", "Cancel"}, false)
		if err != nil {
			return err
		}
		if idx != 0 {
			ui.Warn("Cancelled")
			return nil
		}

		if running, _ := process.IsRunning("nginx"); running {
			ui.Warn("Nginx is running. Run 'gacor stop' before updating.")
			return nil
		}

		oldVersion := cfg.NginxVersion

		err = ui.RunWithProgress(func(onEvent func(ui.ProgressEvent)) error {
			return nginx.DownloadFromURL(latest, versions[latest], func(stage, label string, done, total int64) {
				onEvent(ui.ProgressEvent{Stage: stage, Label: label, Done: done, Total: total})
			})
		})
		if err != nil {
			return err
		}

		cfg.NginxVersion = latest
		if err := cfg.Save(); err != nil {
			return err
		}

		if err := nginx.RemoveVersion(oldVersion); err != nil {
			ui.Warn(fmt.Sprintf("Failed to remove old version %s: %s", oldVersion, err))
		} else {
			ui.Info(fmt.Sprintf("Nginx %s removed", oldVersion))
		}

		ui.Success(fmt.Sprintf("Nginx %s installed and activated", latest))
		return nil
	},
}

func init() {
	nginxCmd.AddCommand(nginxListCmd, nginxUseCmd, nginxInstallCmd, nginxCheckCmd)
}
