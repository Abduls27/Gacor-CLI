package database

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gacor/internal/config"
)

type postgreSQLDriver struct{}

func (postgreSQLDriver) Engine() Engine { return PostgreSQL }

func (postgreSQLDriver) DisplayName() string { return "PostgreSQL" }

func (postgreSQLDriver) DefaultPort() int { return 5432 }

func (postgreSQLDriver) DataDirectoryMarker() string { return "PG_VERSION" }

func (postgreSQLDriver) FetchAvailableVersions() ([]Release, error) { return fetchPostgreSQL() }

func (postgreSQLDriver) FindServer(installDir string) (string, error) {
	return findExecutable(installDir, "postgres.exe")
}

func (postgreSQLDriver) ValidateSetup(name, user string) error {
	if err := validateSetupIdentifiers(name, user); err != nil {
		return err
	}
	if strings.EqualFold(user, "postgres") {
		return fmt.Errorf("nama user postgres sudah digunakan PostgreSQL")
	}
	switch strings.ToLower(name) {
	case "postgres", "template0", "template1":
		return fmt.Errorf("nama database %s sudah digunakan PostgreSQL", name)
	}
	return nil
}

func (driver postgreSQLDriver) InitializeAndStart(ctx context.Context, instance Instance, password string, onEvent func(string, string)) error {
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
	if err := initializePostgreSQL(ctx, installDir, dataDir, password); err != nil {
		return err
	}
	if err := configureAndStartPostgres(ctx, installDir, dataDir, instance, password, func(stage string) {
		notifyDatabaseEvent(onEvent, stage, instance)
	}); err != nil {
		return err
	}
	return markInstalled(instance)
}

func (postgreSQLDriver) Start(instance Instance) error {
	if portInUse(instance.Port) {
		return fmt.Errorf("port %d sudah digunakan; tidak akan memulai database agar tidak terhubung ke instance lain", instance.Port)
	}
	installDir := databaseInstallDir(instance)
	pgCtl, err := findExecutable(installDir, "pg_ctl.exe")
	if err != nil {
		return err
	}
	dataDir := dataDirectory(instance.Engine, instance.Version)
	logFile := filepath.Join(dataDir, "server.log")
	cmd := exec.Command(pgCtl, "start", "-D", dataDir, "-l", logFile,
		"-o", fmt.Sprintf("-p %d -h 127.0.0.1", instance.Port), "-w")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(strings.ToLower(string(output)), "already running") {
		return fmt.Errorf("gagal menjalankan PostgreSQL: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := waitForPort(context.Background(), instance.Port); err != nil {
		if stopErr := stopPostgresServer(pgCtl, dataDir); stopErr != nil {
			return fmt.Errorf("%w; gagal menghentikan instance PostgreSQL: %v", err, stopErr)
		}
		return err
	}
	return nil
}

func (postgreSQLDriver) Stop(instance Instance, _ string) error {
	installDir := databaseInstallDir(instance)
	pgCtl, err := findExecutable(installDir, "pg_ctl.exe")
	if err != nil {
		return err
	}
	dataDir := dataDirectory(instance.Engine, instance.Version)
	if err := stopPostgresServer(pgCtl, dataDir); err != nil {
		return fmt.Errorf("gagal menghentikan PostgreSQL: %w", err)
	}
	return waitForPortClosed(instance.Port)
}

func (postgreSQLDriver) CreateDatabase(instance Instance, name, adminPassword string) error {
	if err := (postgreSQLDriver{}).ValidateSetup(name, instance.User); err != nil {
		return err
	}
	sql := fmt.Sprintf("CREATE DATABASE %s OWNER %s;\n",
		quotePostgresIdentifier(name), quotePostgresIdentifier(instance.User))
	return runPostgreSQLSQL(instance, adminPassword, sql, "membuat database "+name)
}

func (postgreSQLDriver) DeleteDatabase(instance Instance, name, adminPassword string) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("nama database tidak valid: %s", name)
	}
	switch strings.ToLower(name) {
	case "postgres", "template0", "template1":
		return fmt.Errorf("database %s tidak dapat dihapus", name)
	}
	sql := "DROP DATABASE " + quotePostgresIdentifier(name) + ";\n"
	return runPostgreSQLSQL(instance, adminPassword, sql, "menghapus database "+name)
}

func (postgreSQLDriver) ListDatabases(instance Instance, adminPassword string) ([]DatabaseInfo, error) {
	installDir := databaseInstallDir(instance)
	psql, err := findExecutable(installDir, "psql.exe")
	if err != nil {
		return nil, err
	}
	query := "SELECT d.datname, r.rolname FROM pg_database d " +
		"JOIN pg_roles r ON r.oid = d.datdba " +
		"WHERE NOT d.datistemplate AND d.datname <> 'postgres' ORDER BY d.datname"
	cmd := exec.Command(psql, "--no-psqlrc", "--set=ON_ERROR_STOP=1", "--tuples-only",
		"--no-align", "--field-separator=\t", "--host=127.0.0.1",
		fmt.Sprintf("--port=%d", instance.Port), "--username=postgres",
		"--dbname=postgres", "--command", query)
	cmd.Env = environmentWith("PGPASSWORD", adminPassword)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar database: %w: %s", err, redactDatabasePassword(string(output), adminPassword))
	}
	return parseDatabaseList(string(output), instance.Port)
}

func (postgreSQLDriver) IsRunning(instance Instance) bool {
	pgCtl, err := findExecutable(databaseInstallDir(instance), "pg_ctl.exe")
	if err != nil {
		return false
	}
	cmd := exec.Command(pgCtl, "status", "-D", dataDirectory(instance.Engine, instance.Version))
	return cmd.Run() == nil
}

func runPostgreSQLSQL(instance Instance, password, sql, action string) error {
	psql, err := findExecutable(databaseInstallDir(instance), "psql.exe")
	if err != nil {
		return err
	}
	cmd := exec.Command(psql, "--no-psqlrc", "--set=ON_ERROR_STOP=1",
		"--host=127.0.0.1", fmt.Sprintf("--port=%d", instance.Port),
		"--username=postgres", "--dbname=postgres")
	cmd.Env = environmentWith("PGPASSWORD", password)
	cmd.Stdin = strings.NewReader(sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal %s: %w: %s", action, err, redactDatabasePassword(string(output), password))
	}
	return nil
}

func initializePostgreSQL(ctx context.Context, installDir, dataDir, password string) error {
	initdb, err := findExecutable(installDir, "initdb.exe")
	if err != nil {
		return err
	}
	authHost := "scram-sha-256"
	if password == "" {
		authHost = "trust"
	}
	args := []string{
		"-D", dataDir,
		"-U", "postgres",
		"--auth-host=" + authHost,
		"--auth-local=" + authHost,
		"--encoding=UTF8",
	}
	if password != "" {
		passwordFile, err := os.CreateTemp(config.BaseDir(), "gacor-pg-password-*")
		if err != nil {
			return err
		}
		passwordPath := passwordFile.Name()
		defer os.Remove(passwordPath)
		if err := passwordFile.Chmod(0600); err != nil {
			passwordFile.Close()
			return err
		}
		if _, err := io.WriteString(passwordFile, password+"\n"); err != nil {
			passwordFile.Close()
			return err
		}
		if err := passwordFile.Close(); err != nil {
			return err
		}
		args = append(args, "--pwfile="+passwordPath)
	}
	cmd := exec.CommandContext(ctx, initdb, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal inisialisasi PostgreSQL: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func configureAndStartPostgres(ctx context.Context, installDir, dataDir string, instance Instance, password string, notify func(string)) error {
	pgCtl, err := findExecutable(installDir, "pg_ctl.exe")
	if err != nil {
		return err
	}
	logFile := filepath.Join(dataDir, "server.log")
	start := exec.CommandContext(ctx, pgCtl,
		"start",
		"-D", dataDir,
		"-l", logFile,
		"-o", fmt.Sprintf("-p %d -h 127.0.0.1", instance.Port),
		"-w",
	)
	output, err := start.CombinedOutput()
	if err != nil {
		if stopErr := stopPostgresServer(pgCtl, dataDir); stopErr != nil {
			return fmt.Errorf("gagal menjalankan PostgreSQL: %w: %s; gagal menghentikan instance: %v",
				err, strings.TrimSpace(string(output)), stopErr)
		}
		return fmt.Errorf("gagal menjalankan PostgreSQL: %w: %s", err, strings.TrimSpace(string(output)))
	}
	notify("start")
	if err := waitForPort(ctx, instance.Port); err != nil {
		if stopErr := stopPostgresServer(pgCtl, dataDir); stopErr != nil {
			return fmt.Errorf("%w; gagal menghentikan instance PostgreSQL: %v", err, stopErr)
		}
		return err
	}

	notify("configure")
	psql, err := findExecutable(installDir, "psql.exe")
	if err != nil {
		if stopErr := stopPostgresServer(pgCtl, dataDir); stopErr != nil {
			return fmt.Errorf("%w; gagal menghentikan instance PostgreSQL: %v", err, stopErr)
		}
		return err
	}
	createRole := fmt.Sprintf("CREATE ROLE %s LOGIN", quotePostgresIdentifier(instance.User))
	if password != "" {
		createRole += fmt.Sprintf(" PASSWORD '%s'", strings.ReplaceAll(password, "'", "''"))
	}
	sql := fmt.Sprintf("%s;\nCREATE DATABASE %s OWNER %s;\n",
		createRole,
		quotePostgresIdentifier(instance.Name),
		quotePostgresIdentifier(instance.User),
	)
	cmd := exec.CommandContext(ctx, psql,
		"--no-psqlrc",
		"--host=127.0.0.1",
		fmt.Sprintf("--port=%d", instance.Port),
		"--username=postgres",
		"--dbname=postgres",
	)
	cmd.Env = environmentWith("PGPASSWORD", password)
	cmd.Stdin = strings.NewReader(sql)
	output, err = cmd.CombinedOutput()
	if err != nil {
		details := strings.ReplaceAll(strings.TrimSpace(string(output)), password, "[redacted]")
		if stopErr := stopPostgresServer(pgCtl, dataDir); stopErr != nil {
			return fmt.Errorf("gagal membuat database/user PostgreSQL: %w: %s; gagal menghentikan instance: %v",
				err, details, stopErr)
		}
		return fmt.Errorf("gagal membuat database/user PostgreSQL: %w: %s", err, details)
	}
	if err := ctx.Err(); err != nil {
		if stopErr := stopPostgresServer(pgCtl, dataDir); stopErr != nil {
			return fmt.Errorf("%w; gagal menghentikan instance PostgreSQL: %v", err, stopErr)
		}
		return err
	}
	return nil
}

func stopPostgresServer(pgCtl, dataDir string) error {
	cmd := exec.Command(pgCtl, "stop", "-D", dataDir, "-m", "fast", "-w")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func quotePostgresIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func fetchPostgreSQL() ([]Release, error) {
	const indexURL = "https://ftp.postgresql.org/pub/source/"
	body, err := fetchText(indexURL)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil versi PostgreSQL dari ftp.postgresql.org: %w", err)
	}

	directoryRE := regexp.MustCompile(`href="v(\d+\.\d+(?:\.\d+)?)/"`)
	latestByMajor := map[int]string{}
	for _, match := range directoryRE.FindAllStringSubmatch(body, -1) {
		version := match[1]
		major, err := strconv.Atoi(strings.Split(version, ".")[0])
		if err != nil || major < 14 {
			continue
		}
		current := latestByMajor[major]
		if current == "" || compareVersion(version, current) > 0 {
			latestByMajor[major] = version
		}
	}

	var releases []Release
	for major, version := range latestByMajor {
		if major < 14 {
			continue
		}
		archiveURL := "https://get.enterprisedb.com/postgresql/postgresql-" + version + "-1-windows-x64-binaries.zip"
		ok, err := archiveAvailable(archiveURL)
		if err != nil {
			return nil, fmt.Errorf("gagal memeriksa arsip PostgreSQL %s: %w", version, err)
		}
		if ok {
			releases = append(releases, Release{Version: version, URL: archiveURL})
		}
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("tidak menemukan arsip binary Windows x64 PostgreSQL yang tersedia dari EDB")
	}
	sortReleases(releases)
	return releases, nil
}
