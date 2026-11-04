package cmd

import (
	"fmt"
	"gacor/internal/config"
	"gacor/internal/project"
	"gacor/internal/ui"
	"strings"

	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage Laravel project folders",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		actions := []string{"View project list"}
		if len(cfg.ProjectFolders) == 0 {
			actions = []string{"Add a discovery folder", actions[0]}
		} else {
			actions = []string{"Change discovery folder", actions[0]}
		}

		index, err := ui.SelectIndexed("Manage Laravel projects:", actions)
		if err != nil {
			return err
		}
		if index < 0 {
			ui.Warn("Cancelled")
			return nil
		}

		if index == len(actions)-1 {
			return listProjects(cfg)
		}
		if len(cfg.ProjectFolders) == 0 {
			return addProjectFolder(cfg, "")
		}
		return changeProjectFolder(cfg, "", "")
	},
}

var projectAddCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Add a Laravel project folder",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		path := ""
		if len(args) == 1 {
			path = args[0]
		}
		return addProjectFolder(cfg, path)
	},
}

var projectChangeCmd = &cobra.Command{
	Use:   "change [old-path new-path]",
	Short: "Change the Laravel project discovery folder",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 && len(args) != 2 {
			return fmt.Errorf("use `gacor project change` or `gacor project change <old-path> <new-path>`")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		oldPath, newPath := "", ""
		if len(args) == 2 {
			oldPath, newPath = args[0], args[1]
		}
		return changeProjectFolder(cfg, oldPath, newPath)
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all discovered Laravel projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return listProjects(cfg)
	},
}

func init() {
	projectCmd.AddCommand(projectAddCmd, projectChangeCmd, projectListCmd)
}

func updateProjectMenu() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	hasFolders := len(cfg.ProjectFolders) > 0
	projectAddCmd.Hidden = hasFolders
	projectChangeCmd.Hidden = !hasFolders
	return nil
}

func addProjectFolder(cfg *config.Config, path string) error {
	if path == "" {
		var err error
		path, err = ui.PromptInput("Laravel project parent folder:", false)
		if err == ui.ErrPromptCancelled {
			ui.Warn("Cancelled")
			return nil
		}
		if err != nil {
			return err
		}
		path = strings.TrimSpace(path)
	}
	if path == "" {
		ui.Warn("Folder path cannot be empty")
		return nil
	}
	if err := project.AddFolder(cfg, path); err != nil {
		ui.Error(err.Error())
		return nil
	}
	ui.Success(fmt.Sprintf("Folder added: %s", path))
	return nil
}

func changeProjectFolder(cfg *config.Config, oldPath, newPath string) error {
	if len(cfg.ProjectFolders) == 0 {
		ui.Warn("There are no discovery folders to change. Use add first.")
		return nil
	}

	if oldPath == "" {
		labels := append([]string(nil), cfg.ProjectFolders...)
		index, err := ui.SelectIndexedWithSearch("Select the discovery folder to change:", labels, false)
		if err != nil {
			return err
		}
		if index < 0 {
			ui.Warn("Cancelled")
			return nil
		}
		oldPath = cfg.ProjectFolders[index]
	}
	if newPath == "" {
		var err error
		newPath, err = ui.PromptInput("Replacement discovery folder:", false)
		if err == ui.ErrPromptCancelled {
			ui.Warn("Cancelled")
			return nil
		}
		if err != nil {
			return err
		}
		newPath = strings.TrimSpace(newPath)
	}
	if newPath == "" {
		ui.Warn("Folder path cannot be empty")
		return nil
	}
	if err := project.ChangeFolder(cfg, oldPath, newPath); err != nil {
		ui.Error(err.Error())
		return nil
	}
	ui.Success(fmt.Sprintf("Discovery folder changed: %s", newPath))
	return nil
}

func listProjects(cfg *config.Config) error {
	projects := project.ListProjects(cfg)
	if len(projects) == 0 {
		ui.Warn("No Laravel projects were found.")
		if len(cfg.ProjectFolders) == 0 {
			ui.Info("Add a parent folder with: gacor project add <path>")
		}
		return nil
	}
	fmt.Printf("Found %d Laravel project(s):\n", len(projects))
	for _, p := range projects {
		fmt.Printf("  → %s\n", p)
	}
	return nil
}
