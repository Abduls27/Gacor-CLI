package cmd

import (
	"fmt"
	"gacor/internal/config"
	"gacor/internal/php"
	"gacor/internal/process"
	"gacor/internal/ui"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
)

var phpCmd = &cobra.Command{
	Use:   "php",
	Short: "Manage PHP versions",
}

func startPHPService(cfg *config.Config) error {
	if running, _ := process.IsRunning("php"); running {
		if err := process.Stop("php"); err != nil {
			return fmt.Errorf("failed to stop the running PHP process: %w", err)
		}
	}

	phpExe := php.CGIPath(cfg.PHPVersion)
	phpDir := filepath.Dir(phpExe)
	workers := cfg.PHPWorkers
	if workers == 0 {
		workers = 4
	}
	phpEnv := []string{
		fmt.Sprintf("PHP_FCGI_CHILDREN=%d", workers),
		"PHP_FCGI_MAX_REQUESTS=1000",
	}
	if err := process.StartBackground("php", phpExe, []string{"-b", "127.0.0.1:9000"}, phpDir, phpEnv); err != nil {
		return fmt.Errorf("failed to start PHP %s: %w", cfg.PHPVersion, err)
	}
	return nil
}

var phpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available PHP versions (from windows.php.net)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Info("Fetching PHP versions from windows.php.net...")
		versions, err := php.FetchAvailableVersions()
		if err != nil {
			ui.Error(err.Error())
			return nil
		}
		cfg, _ := config.Load()
		for _, v := range php.SortedVersions(versions) {
			installed := ""
			active := ""
			if php.IsInstalled(v) {
				installed = " [installed]"
			}
			if cfg.PHPVersion == v {
				active = " ◀ active"
			}
			fmt.Printf("  %s%s%s\n", v, installed, active)
		}
		return nil
	},
}

var phpUseCmd = &cobra.Command{
	Use:   "use",
	Short: "Select a PHP version using the arrow keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Info("Fetching PHP versions from windows.php.net...")
		versions, err := php.FetchAvailableVersions()
		if err != nil {
			ui.Error(err.Error())
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		// Mark installed versions in the selector labels.
		items := php.SortedVersions(versions)
		labels := make([]string, len(items))
		for i, v := range items {
			labels[i] = v
			if php.IsInstalled(v) {
				labels[i] += " (installed)"
			}
			if cfg.PHPVersion == v {
				labels[i] += " ◀ active"
			}
		}

		idx, err := ui.SelectIndexed("Select a PHP version:", labels)
		if err != nil {
			return err
		}
		if idx < 0 {
			ui.Warn("Cancelled")
			return nil
		}
		chosen := items[idx]

		if !php.IsInstalled(chosen) {
			err := ui.RunWithProgress(func(onEvent func(ui.ProgressEvent)) error {
				return php.DownloadFromURL(chosen, versions[chosen], cfg, func(stage, label string, done, total int64) {
					onEvent(ui.ProgressEvent{Stage: stage, Label: label, Done: done, Total: total})
				})
			})
			if err != nil {
				return err
			}
		} else {
			ui.Info(fmt.Sprintf("PHP %s is already installed; skipping download", chosen))
		}

		re := regexp.MustCompile(`php-(\d+\.\d+\.\d+)`)
		if matches := re.FindStringSubmatch(versions[chosen]); len(matches) >= 2 {
			cfg.PHPFullVersion = matches[1]
		}
		cfg.PHPVersion = chosen

		if err := cfg.Save(); err != nil {
			return err
		}

		if running, _ := process.IsRunning("php"); running {
			ui.Info("Restarting PHP with the selected version...")
			if err := startPHPService(cfg); err != nil {
				return err
			}
			ui.Success(fmt.Sprintf("PHP %s restarted on port 9000", chosen))
		}

		ui.Success(fmt.Sprintf("PHP %s set as the active version", chosen))
		return nil
	},
}

var phpUsePrereleaseCmd = &cobra.Command{
	Use:   "use-pre",
	Short: "Select a pre-release PHP version (RC/dev) — experimental",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Warn("Pre-release build — unstable; use at your own risk.")
		ui.Info("Fetching pre-release versions from windows.php.net/downloads/qa...")
		versions, err := php.FetchPrereleaseVersions()
		if err != nil {
			ui.Error(err.Error())
			return nil
		}
		if len(versions) == 0 {
			ui.Warn("No pre-release versions found")
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		items := php.SortedVersions(versions)
		labels := make([]string, len(items))
		for i, v := range items {
			labels[i] = v
			if php.IsInstalled(v) {
				labels[i] += " (installed)"
			}
			if cfg.PHPVersion == v {
				labels[i] += " ◀ active"
			}
		}

		idx, err := ui.SelectIndexed("Select a pre-release PHP version:", labels)
		if err != nil {
			return err
		}
		if idx < 0 {
			ui.Warn("Cancelled")
			return nil
		}
		chosen := items[idx]

		if !php.IsInstalled(chosen) {
			err := ui.RunWithProgress(func(onEvent func(ui.ProgressEvent)) error {
				return php.DownloadFromURL(chosen, versions[chosen], cfg, func(stage, label string, done, total int64) {
					onEvent(ui.ProgressEvent{Stage: stage, Label: label, Done: done, Total: total})
				})
			})
			if err != nil {
				return err
			}
		} else {
			ui.Info(fmt.Sprintf("PHP %s is already installed; skipping download", chosen))
		}

		cfg.PHPVersion = chosen
		cfg.PHPFullVersion = chosen
		if err := cfg.Save(); err != nil {
			return err
		}

		if running, _ := process.IsRunning("php"); running {
			ui.Info("Restarting PHP with the selected version...")
			if err := startPHPService(cfg); err != nil {
				return err
			}
			ui.Success(fmt.Sprintf("PHP %s restarted on port 9000", chosen))
		}

		ui.Success(fmt.Sprintf("PHP %s (pre-release) set as the active version", chosen))
		return nil
	},
}

var phpInstallCmd = &cobra.Command{
	Use:   "install [version]",
	Short: "Install a specific PHP version",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Info("Fetching download URLs from windows.php.net...")
		versions, err := php.FetchAvailableVersions()
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
			items := php.SortedVersions(versions)
			labels := make([]string, len(items))
			for i, v := range items {
				labels[i] = v
				if php.IsInstalled(v) {
					labels[i] += " (installed)"
				}
				if cfg.PHPVersion == v {
					labels[i] += " ◀ active"
				}
			}
			idx, err := ui.SelectIndexed("Select a PHP version:", labels)
			if err != nil {
				return err
			}
			if idx < 0 {
				ui.Warn("Cancelled")
				return nil
			}
			version = items[idx]
		}
		if php.IsInstalled(version) {
			ui.Info(fmt.Sprintf("PHP %s is already installed", version))
			return nil
		}
		url, ok := versions[version]
		if !ok {
			return fmt.Errorf("PHP version %s was not found", version)
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		return ui.RunWithProgress(func(onEvent func(ui.ProgressEvent)) error {
			return php.DownloadFromURL(version, url, cfg, func(stage, label string, done, total int64) {
				onEvent(ui.ProgressEvent{Stage: stage, Label: label, Done: done, Total: total})
			})
		})
	},
}

var phpCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for and install the latest PHP version",
	RunE: func(cmd *cobra.Command, args []string) error {
		installed, err := php.InstalledVersions()
		if err != nil {
			return err
		}
		if len(installed) == 0 {
			ui.Warn("No PHP versions are installed")
			return nil
		}

		ui.Info("Checking for updates...")
		versions, err := php.FetchAvailableVersions()
		if err != nil {
			ui.Error(err.Error())
			return nil
		}

		type candidate struct{ current, latest string }
		var updates []candidate

		for _, cur := range installed {
			mm := php.MajorMinor(cur)
			latest, ok := php.LatestFor(versions, mm)
			if !ok {
				continue
			}
			if latest == cur {
				ui.Success(fmt.Sprintf("PHP %s is already up to date", cur))
			} else {
				updates = append(updates, candidate{cur, latest})
			}
		}

		if len(updates) == 0 {
			return nil
		}

		// Select the version to update.
		var target candidate
		if len(updates) == 1 {
			u := updates[0]
			label := fmt.Sprintf("Update PHP from %s to %s?", u.current, u.latest)
			idx, err := ui.SelectIndexedWithSearch(label, []string{"Yes, update", "Cancel"}, false)
			if err != nil {
				return err
			}
			if idx != 0 {
				ui.Warn("Cancelled")
				return nil
			}
			target = u
		} else {
			labels := make([]string, 0, len(updates)+1)
			for _, u := range updates {
				labels = append(labels, fmt.Sprintf("%s → %s", php.MajorMinor(u.current), u.latest))
			}
			labels = append(labels, "Cancel")
			idx, err := ui.SelectIndexedWithSearch("Several updates are available; select which one to install:", labels, false)
			if err != nil {
				return err
			}
			if idx < 0 || idx == len(updates) {
				ui.Warn("Cancelled")
				return nil
			}
			target = updates[idx]
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if cfg.PHPVersion == target.current {
			if running, _ := process.IsRunning("php"); running {
				ui.Warn("PHP is running. Run 'gacor stop' before updating.")
				return nil
			}
		}

		err = ui.RunWithProgress(func(onEvent func(ui.ProgressEvent)) error {
			return php.DownloadFromURL(target.latest, versions[target.latest], cfg, func(stage, label string, done, total int64) {
				onEvent(ui.ProgressEvent{Stage: stage, Label: label, Done: done, Total: total})
			})
		})
		if err != nil {
			return err
		}

		wasActive := cfg.PHPVersion == target.current
		if wasActive {
			cfg.PHPVersion = target.latest
			cfg.PHPFullVersion = target.latest
			if err := cfg.Save(); err != nil {
				return err
			}
		}

		if err := php.RemoveVersion(target.current); err != nil {
			ui.Warn(fmt.Sprintf("Failed to remove old version %s: %s", target.current, err))
		} else {
			ui.Info(fmt.Sprintf("PHP %s removed", target.current))
		}

		ui.Success(fmt.Sprintf("PHP %s installed%s", target.latest, map[bool]string{true: " and activated", false: ""}[wasActive]))
		return nil
	},
}

var phpUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update one installed PHP major.minor release to its latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		installed, err := php.InstalledVersions()
		if err != nil {
			return err
		}
		if len(installed) == 0 {
			ui.Warn("No PHP versions are installed")
			return nil
		}

		ui.Info("Checking for updates...")
		versions, err := php.FetchAvailableVersions()
		if err != nil {
			return err
		}

		type candidate struct{ current, latest string }
		var updates []candidate

		for _, cur := range installed {
			mm := php.MajorMinor(cur)
			latest, ok := php.LatestFor(versions, mm)
			if ok && latest != cur {
				updates = append(updates, candidate{cur, latest})
			}
		}

		if len(updates) == 0 {
			ui.Success("All PHP versions are up to date")
			return nil
		}

		// Select which version to update if more than one is available.
		target := updates[0]
		if len(updates) > 1 {
			labels := make([]string, len(updates))
			for i, u := range updates {
				labels[i] = fmt.Sprintf("%s → %s", u.current, u.latest)
			}
			idx, err := ui.SelectIndexed("Select a version to update:", labels)
			if err != nil {
				return err
			}
			if idx < 0 {
				ui.Warn("Cancelled")
				return nil
			}
			target = updates[idx]
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		err = ui.RunWithProgress(func(onEvent func(ui.ProgressEvent)) error {
			return php.DownloadFromURL(target.latest, versions[target.latest], cfg, func(stage, label string, done, total int64) {
				onEvent(ui.ProgressEvent{Stage: stage, Label: label, Done: done, Total: total})
			})
		})
		if err != nil {
			return err
		}

		// Activate the new version if the old version was active.
		if cfg.PHPVersion == target.current {
			cfg.PHPVersion = target.latest
			cfg.PHPFullVersion = target.latest
			if err := cfg.Save(); err != nil {
				return err
			}
		}

		ui.Success(fmt.Sprintf("PHP %s installed", target.latest))
		return nil
	},
}

func init() {
	phpCmd.AddCommand(phpListCmd, phpUseCmd, phpInstallCmd, phpCheckCmd, phpUpdateCmd, phpUsePrereleaseCmd)
}
