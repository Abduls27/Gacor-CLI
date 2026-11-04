package database

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	"gacor/internal/process"
)

type mariaDBDriver struct {
	mysqlDriver
}

func (mariaDBDriver) Engine() Engine { return MariaDB }

func (mariaDBDriver) DisplayName() string { return "MariaDB" }

func (mariaDBDriver) DefaultPort() int { return 3307 }

func (mariaDBDriver) FetchAvailableVersions() ([]Release, error) { return fetchMariaDB() }

func (mariaDBDriver) FindServer(installDir string) (string, error) {
	return findExecutable(installDir, "mariadbd.exe", "mysqld.exe")
}

func (mariaDBDriver) ValidateSetup(name, user string) error {
	return validateMySQLSetup(MariaDB, name, user)
}

func (driver mariaDBDriver) InitializeAndStart(ctx context.Context, instance Instance, password string, onEvent func(string, string)) error {
	if err := validateInstanceForInitialization(ctx, instance); err != nil {
		return err
	}
	installDir := databaseInstallDir(instance)
	if _, err := driver.FindServer(installDir); err != nil {
		return err
	}
	dataDir := dataDirectory(instance.Engine, instance.Version)
	if err := ensureEmptyDataDirectory(instance, dataDir); err != nil {
		return err
	}
	notifyDatabaseEvent(onEvent, "initialize", instance)
	if err := initializeMariaDB(ctx, installDir, dataDir, instance.Port); err != nil {
		return err
	}
	serverPath, err := driver.FindServer(installDir)
	if err != nil {
		return err
	}
	notifyDatabaseEvent(onEvent, "configure", instance)
	if err := writeMySQLConfig(filepath.Dir(filepath.Dir(serverPath)), dataDir, instance); err != nil {
		return err
	}
	notifyDatabaseEvent(onEvent, "start", instance)
	if err := startMySQLServer(serverPath, installDir, instance); err != nil {
		return err
	}
	if err := waitForPort(ctx, instance.Port); err != nil {
		_ = process.Stop("database")
		return err
	}
	notifyDatabaseEvent(onEvent, "configure", instance)
	if err := createMySQLDatabase(ctx, installDir, instance, password); err != nil {
		if stopErr := runMySQLAdmin(installDir, instance, "root", password, "shutdown"); stopErr != nil {
			_ = process.Stop("database")
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		if stopErr := runMySQLAdmin(installDir, instance, "root", password, "shutdown"); stopErr != nil {
			_ = process.Stop("database")
			return fmt.Errorf("%w; failed to stop MariaDB after cancellation: %v", err, stopErr)
		}
		return err
	}
	return markInstalled(instance)
}

func initializeMariaDB(ctx context.Context, installDir, dataDir string, port int) error {
	installer, err := findExecutable(installDir, "mariadb-install-db.exe", "mysql_install_db.exe")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, installer,
		"--datadir="+dataDir,
		"--port="+fmt.Sprint(port),
		"--silent",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to initialize MariaDB: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

type mariaRoot struct {
	MajorReleases []struct {
		ReleaseID     string `json:"release_id"`
		ReleaseStatus string `json:"release_status"`
	} `json:"major_releases"`
}

type mariaTrack struct {
	Releases map[string]struct {
		Files []struct {
			FileName        string `json:"file_name"`
			FileDownloadURL string `json:"file_download_url"`
			OS              string `json:"os"`
			CPU             string `json:"cpu"`
		} `json:"files"`
	} `json:"releases"`
}

func fetchMariaDB() ([]Release, error) {
	const apiURL = "https://downloads.mariadb.org/rest-api/mariadb/"
	var root mariaRoot
	if err := fetchJSON(apiURL, &root); err != nil {
		return nil, fmt.Errorf("failed to fetch MariaDB versions from downloads.mariadb.org: %w", err)
	}

	seen := map[string]bool{}
	var releases []Release
	for _, track := range root.MajorReleases {
		if track.ReleaseStatus != "Stable" {
			continue
		}
		var data mariaTrack
		if err := fetchJSON(apiURL+url.PathEscape(track.ReleaseID)+"/", &data); err != nil {
			return nil, fmt.Errorf("failed to read MariaDB metadata for %s: %w", track.ReleaseID, err)
		}
		for version, info := range data.Releases {
			for _, file := range info.Files {
				if file.OS != "Windows" || file.CPU != "x86_64" || !strings.HasSuffix(strings.ToLower(file.FileName), "-winx64.zip") {
					continue
				}
				if seen[version] {
					break
				}
				downloadURL := strings.Replace(file.FileDownloadURL, "http://", "https://", 1)
				releases = append(releases, Release{Version: version, URL: downloadURL})
				seen[version] = true
				break
			}
		}
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("the official MariaDB API did not return a Windows x64 archive")
	}
	return latestPerSeries(releases), nil
}
