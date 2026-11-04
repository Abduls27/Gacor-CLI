package project

import (
	"fmt"
	"gacor/internal/config"
	"os"
	"path/filepath"
)

// ListProjects scans all registered project folders and returns Laravel projects
func ListProjects(cfg *config.Config) []string {
	var projects []string
	for _, folder := range cfg.ProjectFolders {
		entries, err := os.ReadDir(folder)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			// Check if it's a Laravel project (has artisan file)
			artisan := filepath.Join(folder, e.Name(), "artisan")
			if _, err := os.Stat(artisan); err == nil {
				projects = append(projects, filepath.Join(folder, e.Name()))
			}
		}
	}
	return projects
}

func AddFolder(cfg *config.Config, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("directory not found: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	// Check duplicate
	for _, f := range cfg.ProjectFolders {
		if f == path {
			return fmt.Errorf("directory is already registered")
		}
	}
	cfg.ProjectFolders = append(cfg.ProjectFolders, path)
	return cfg.Save()
}

func ChangeFolder(cfg *config.Config, oldPath, newPath string) error {
	info, err := os.Stat(newPath)
	if err != nil {
		return fmt.Errorf("directory not found: %s", newPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", newPath)
	}

	for i, f := range cfg.ProjectFolders {
		if f == oldPath {
			for j, existing := range cfg.ProjectFolders {
				if j != i && existing == newPath {
					return fmt.Errorf("directory is already registered: %s", newPath)
				}
			}
			cfg.ProjectFolders[i] = newPath
			return cfg.Save()
		}
	}
	return fmt.Errorf("directory not found in configuration: %s", oldPath)
}
