package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	PHPVersion      string   `json:"php_version"`
	PHPFullVersion  string   `json:"php_full_version"`
	NginxVersion    string   `json:"nginx_version"`
	DatabaseEngine  string   `json:"database_engine"`
	DatabaseVersion string   `json:"database_version"`
	DatabaseName    string   `json:"database_name"`
	DatabaseUser    string   `json:"database_user"`
	DatabasePort    int      `json:"database_port"`
	ProjectFolders  []string `json:"project_folders"`
	PHPWorkers      int      `json:"php_workers"`
	PHPUploadMax    string   `json:"php_upload_max"`
	PHPMemoryLimit  string   `json:"php_memory_limit"`
	PHPTimezone     string   `json:"php_timezone"`
}

func BaseDir() string {
	if d := os.Getenv("GACOR_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gacor")
}

func (c *Config) Save() error {
	if err := os.MkdirAll(BaseDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0644)
}

func BinDir() string {
	return filepath.Join(BaseDir(), "bin")
}

func PHPDir() string {
	return filepath.Join(BinDir(), "php")
}

func NginxDir() string {
	return filepath.Join(BinDir(), "nginx")
}

func DatabaseDir() string {
	return filepath.Join(BinDir(), "database")
}

func ConfigPath() string {
	return filepath.Join(BaseDir(), "gacor.json")
}

func Load() (*Config, error) {
	cfg := &Config{
		PHPWorkers:     4,
		PHPUploadMax:   "100M",
		PHPMemoryLimit: "256M",
		PHPTimezone:    "Asia/Jakarta",
	}
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	err = json.Unmarshal(data, cfg)
	return cfg, err
}

func EnsureDirs() error {
	for _, dir := range []string{BinDir(), PHPDir(), NginxDir()} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}
