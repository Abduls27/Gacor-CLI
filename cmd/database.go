package cmd

import (
	"context"
	"errors"
	"fmt"
	"gacor/internal/config"
	"gacor/internal/database"
	"gacor/internal/ui"
	"strings"

	"github.com/spf13/cobra"
)

var databaseInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Select and install MySQL, MariaDB, or PostgreSQL",
	RunE: func(cmd *cobra.Command, args []string) error {
		engineLabels := make([]string, 0, len(database.Engines()))
		engines := database.Engines()
		for _, engine := range engines {
			engineLabels = append(engineLabels, engine.String())
		}
		engineIndex, err := ui.SelectIndexed("Select a database:", engineLabels)
		if err != nil {
			return err
		}
		if engineIndex < 0 {
			ui.Warn("Cancelled")
			return nil
		}
		engine := engines[engineIndex]

		ui.Info("Fetching official " + engine.String() + " versions...")
		releases, err := database.FetchAvailableVersions(engine)
		if err != nil {
			return err
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		labels := make([]string, len(releases))
		for i, release := range releases {
			labels[i] = release.Version
			if database.IsInstalled(engine, release.Version) {
				labels[i] += " (installed)"
			}
			if cfg.DatabaseEngine == string(engine) && cfg.DatabaseVersion == release.Version {
				labels[i] += " ◀ active"
			}
		}
		releaseIndex, err := ui.SelectIndexed("Select a "+engine.String()+" version:", labels)
		if err != nil {
			return err
		}
		if releaseIndex < 0 {
			ui.Warn("Cancelled")
			return nil
		}
		release := releases[releaseIndex]
		if database.IsInstalled(engine, release.Version) {
			ui.Info(fmt.Sprintf("%s %s is already installed. Use `gacor database start|stop|status` to manage it.", engine.String(), release.Version))
			return nil
		}

		instance, password, err := promptDatabaseSetup(engine, release.Version)
		if err != nil {
			if errors.Is(err, ui.ErrPromptCancelled) {
				ui.Warn("Cancelled")
				return nil
			}
			return err
		}
		if database.PortInUse(instance.Port) {
			return fmt.Errorf("port %d is already in use; stop other services on this port before installing", instance.Port)
		}

		err = ui.RunWithProgressContext(func(ctx context.Context, onEvent func(ui.ProgressEvent)) error {
			_, installErr := database.InstallArchive(ctx, release, engine, func(stage, label string, done, total int64) {
				onEvent(ui.ProgressEvent{Stage: stage, Label: label, Done: done, Total: total})
			})
			if installErr != nil {
				return installErr
			}
			if err := saveDatabaseSelection(cfg, instance); err != nil {
				return err
			}
			return database.InitializeAndStart(ctx, instance, password, func(stage, label string) {
				onEvent(ui.ProgressEvent{Stage: stage, Label: label})
			})
		})
		if err != nil {
			return err
		}

		ui.Success(fmt.Sprintf("%s %s is ready and running", engine.String(), release.Version))
		ui.Label("Host", "127.0.0.1")
		ui.Label("Port", fmt.Sprint(instance.Port))
		ui.Label("Database", instance.Name)
		ui.Label("User", instance.User)
		if password == "" {
			ui.Warn("The database account was created without a password; use it only for local development.")
		} else {
			ui.Info("The password is not stored in the GACOR configuration. Save the newly created password.")
		}
		return nil
	},
}

var databaseCmd = &cobra.Command{
	Use:     "database",
	Aliases: []string{"db"},
	Short:   "Manage the local database",
}

var databaseStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the selected database",
	RunE: func(cmd *cobra.Command, args []string) error {
		instance, err := configuredDatabase()
		if err != nil {
			return err
		}
		if database.IsRunning(instance) {
			ui.Info(instance.Engine.String() + " is already running")
			return nil
		}
		if err := database.Start(instance); err != nil {
			return err
		}
		ui.Success(instance.Engine.String() + " is running at 127.0.0.1:" + fmt.Sprint(instance.Port))
		return nil
	},
}

var databaseShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "View and manage databases on the selected server",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instance, err := configuredDatabase()
		if err != nil {
			if errors.Is(err, errNoDatabaseConfigured) {
				ui.Warn("No database has been selected. Run: gacor database install")
				return nil
			}
			return err
		}
		if !database.IsRunning(instance) {
			ui.Info(instance.Engine.String() + " is not running; starting...")
			if err := database.Start(instance); err != nil {
				return err
			}
			ui.Success(instance.Engine.String() + " is running at 127.0.0.1:" + fmt.Sprint(instance.Port))
		}
		if len(args) == 0 {
			return manageDatabases(instance)
		}
		if err := database.ValidateSetup(instance.Engine, args[0], instance.User); err != nil {
			return err
		}
		password, err := ui.PromptInput("Database administrator password:", true)
		if err != nil {
			if errors.Is(err, ui.ErrPromptCancelled) {
				ui.Warn("Cancelled")
				return nil
			}
			return err
		}
		if err := database.CreateDatabase(instance, args[0], password); err != nil {
			return err
		}
		ui.Success(fmt.Sprintf("Database %s created successfully; use the %s user", args[0], instance.User))
		return nil
	},
}

func manageDatabases(instance database.Instance) error {
	password, err := ui.PromptInput("Database administrator password:", true)
	if err != nil {
		if errors.Is(err, ui.ErrPromptCancelled) {
			return nil
		}
		return err
	}

	for {
		databases, err := database.ListDatabases(instance, password)
		if err != nil {
			return err
		}
		entries := make([]ui.DatabaseEntry, len(databases))
		for i, item := range databases {
			entries[i] = ui.DatabaseEntry{Name: item.Name, User: item.User, Port: item.Port}
		}
		result, err := ui.DatabaseManager(instance.Engine.String(), instance.Version,
			instance.User, instance.Port, entries)
		if err != nil {
			return err
		}

		switch result.Action {
		case ui.DatabaseActionCreate:
			name, err := ui.PromptInput("New database name:", false)
			if err != nil {
				if errors.Is(err, ui.ErrPromptCancelled) {
					continue
				}
				return err
			}
			if err := database.CreateDatabase(instance, name, password); err != nil {
				ui.Error(err.Error())
				continue
			}
			ui.Success(fmt.Sprintf("Database %s created successfully; use the %s user", name, instance.User))

		case ui.DatabaseActionDelete:
			confirmed, err := ui.Confirm(fmt.Sprintf("Delete database %q? This action cannot be undone.", result.DatabaseName))
			if err != nil {
				if errors.Is(err, ui.ErrPromptCancelled) {
					continue
				}
				return err
			}
			if !confirmed {
				continue
			}
			if err := database.DeleteDatabase(instance, result.DatabaseName, password); err != nil {
				ui.Error(err.Error())
				continue
			}
			ui.Success(fmt.Sprintf("Database %s deleted successfully", result.DatabaseName))

		default:
			return nil
		}
	}
}

var databaseStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running database",
	RunE: func(cmd *cobra.Command, args []string) error {
		instance, err := configuredDatabase()
		if err != nil {
			return err
		}
		if !database.IsRunning(instance) {
			ui.Info(instance.Engine.String() + " is not running")
			return nil
		}
		if err := stopDatabase(instance); err != nil {
			if errors.Is(err, ui.ErrPromptCancelled) {
				ui.Warn("Cancelled")
				return nil
			}
			return err
		}
		ui.Success(instance.Engine.String() + " stopped")
		return nil
	},
}

var databaseStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the selected database status",
	RunE: func(cmd *cobra.Command, args []string) error {
		instance, err := configuredDatabase()
		if err != nil {
			if errors.Is(err, errNoDatabaseConfigured) {
				ui.Warn("No database has been selected. Run: gacor database install")
				return nil
			}
			return err
		}
		state := "STOPPED"
		if database.IsRunning(instance) {
			state = "RUNNING"
		}
		ui.Label("Database", instance.Engine.String()+" "+instance.Version)
		ui.Label("Status", state)
		ui.Label("Host", fmt.Sprintf("127.0.0.1:%d", instance.Port))
		ui.Label("Database", instance.Name)
		ui.Label("User", instance.User)
		return nil
	},
}

var errNoDatabaseConfigured = errors.New("database is not configured")

func promptDatabaseSetup(engine database.Engine, version string) (database.Instance, string, error) {
	name, err := ui.PromptInput("New database name:", false)
	if err != nil {
		return database.Instance{}, "", err
	}
	user, err := ui.PromptInput("Database username:", false)
	if err != nil {
		return database.Instance{}, "", err
	}
	password, err := ui.PromptInput("Database user password (leave blank for no password):", true)
	if err != nil {
		return database.Instance{}, "", err
	}
	confirmation, err := ui.PromptInput("Confirm password (leave blank for no password):", true)
	if err != nil {
		return database.Instance{}, "", err
	}
	if strings.TrimSpace(name) == "" || strings.TrimSpace(user) == "" {
		return database.Instance{}, "", fmt.Errorf("database name and username are required")
	}
	if err := database.ValidateSetup(engine, name, user); err != nil {
		return database.Instance{}, "", err
	}
	if err := validateDatabasePassword(password); err != nil {
		return database.Instance{}, "", err
	}
	if password != confirmation {
		return database.Instance{}, "", fmt.Errorf("passwords do not match")
	}
	return database.Instance{
		Engine:  engine,
		Version: version,
		Name:    name,
		User:    user,
		Port:    database.DefaultPort(engine),
	}, password, nil
}

func validateDatabasePassword(password string) error {
	if password != "" && len(password) < 4 {
		return fmt.Errorf("password must be at least 4 characters or left blank")
	}
	if strings.ContainsAny(password, "\x00\r\n") {
		return fmt.Errorf("password must not contain NUL or newline characters")
	}
	return nil
}

func configuredDatabase() (database.Instance, error) {
	cfg, err := config.Load()
	if err != nil {
		return database.Instance{}, err
	}
	if cfg.DatabaseEngine == "" || cfg.DatabaseVersion == "" || cfg.DatabasePort == 0 {
		return database.Instance{}, errNoDatabaseConfigured
	}
	return database.Instance{
		Engine:  database.Engine(cfg.DatabaseEngine),
		Version: cfg.DatabaseVersion,
		Name:    cfg.DatabaseName,
		User:    cfg.DatabaseUser,
		Port:    cfg.DatabasePort,
	}, nil
}

func saveDatabaseSelection(cfg *config.Config, instance database.Instance) error {
	cfg.DatabaseEngine = string(instance.Engine)
	cfg.DatabaseVersion = instance.Version
	cfg.DatabaseName = instance.Name
	cfg.DatabaseUser = instance.User
	cfg.DatabasePort = instance.Port
	return cfg.Save()
}

func stopDatabase(instance database.Instance) error {
	return database.Stop(instance, "")
}

func init() {
	databaseCmd.AddCommand(databaseInstallCmd, databaseStartCmd, databaseShowCmd, databaseStopCmd, databaseStatusCmd)
}
