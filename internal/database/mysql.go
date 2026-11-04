package database

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gacor/internal/process"
)

type mysqlDriver struct{}

func (mysqlDriver) Engine() Engine { return MySQL }

func (mysqlDriver) DisplayName() string { return "MySQL" }

func (mysqlDriver) DefaultPort() int { return 3306 }

func (mysqlDriver) DataDirectoryMarker() string { return "mysql" }

func (mysqlDriver) FetchAvailableVersions() ([]Release, error) { return fetchMySQL() }

func (mysqlDriver) FindServer(installDir string) (string, error) {
	return findExecutable(installDir, "mysqld.exe")
}

func (mysqlDriver) ValidateSetup(name, user string) error {
	return validateMySQLSetup(MySQL, name, user)
}

func (driver mysqlDriver) InitializeAndStart(ctx context.Context, instance Instance, password string, onEvent func(string, string)) error {
	if err := validateInstanceForInitialization(ctx, instance); err != nil {
		return err
	}
	installDir := databaseInstallDir(instance)
	serverPath, err := driver.FindServer(installDir)
	if err != nil {
		return err
	}
	dataDir := dataDirectory(instance.Engine, instance.Version)
	if err := ensureEmptyDataDirectory(instance, dataDir); err != nil {
		return err
	}
	notifyDatabaseEvent(onEvent, "initialize", instance)
	if err := initializeMySQL(ctx, serverPath, executableDirectory(serverPath), dataDir); err != nil {
		return err
	}
	notifyDatabaseEvent(onEvent, "configure", instance)
	if err := writeMySQLConfig(executableDirectory(serverPath), dataDir, instance); err != nil {
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
			return fmt.Errorf("%w; failed to stop MySQL after cancellation: %v", err, stopErr)
		}
		return err
	}
	return markInstalled(instance)
}

func (mysqlDriver) Start(instance Instance) error {
	return startMySQLInstance(instance)
}

func (mysqlDriver) Stop(instance Instance, password string) error {
	return stopMySQLInstance(instance, password)
}

func (mysqlDriver) CreateDatabase(instance Instance, name, adminPassword string) error {
	return createMySQLDatabaseByName(instance, name, adminPassword)
}

func (mysqlDriver) DeleteDatabase(instance Instance, name, adminPassword string) error {
	return deleteMySQLDatabase(instance, name, adminPassword)
}

func (mysqlDriver) ListDatabases(instance Instance, adminPassword string) ([]DatabaseInfo, error) {
	return listMySQLDatabases(instance, adminPassword)
}

func (mysqlDriver) IsRunning(instance Instance) bool {
	running, _ := process.IsRunning("database")
	return running && portInUse(instance.Port)
}

func startMySQLInstance(instance Instance) error {
	installDir := databaseInstallDir(instance)
	driver, err := driverFor(instance.Engine)
	if err != nil {
		return err
	}
	serverPath, err := driver.FindServer(installDir)
	if err != nil {
		return err
	}
	if portInUse(instance.Port) {
		return fmt.Errorf("port %d is already in use; refusing to start the database to avoid connecting to another instance", instance.Port)
	}
	if err := writeMySQLConfig(executableDirectory(serverPath), dataDirectory(instance.Engine, instance.Version), instance); err != nil {
		return fmt.Errorf("failed to update %s configuration: %w", instance.Engine, err)
	}
	if err := startMySQLServer(serverPath, installDir, instance); err != nil {
		return err
	}
	if err := waitForPort(context.Background(), instance.Port); err != nil {
		_ = process.Stop("database")
		return err
	}
	return nil
}

func stopMySQLInstance(instance Instance, password string) error {
	installDir := databaseInstallDir(instance)
	if err := runMySQLAdmin(installDir, instance, "root", password, "shutdown"); err != nil {
		if running, _ := process.IsRunning("database"); running {
			if stopErr := process.Stop("database"); stopErr != nil {
				return fmt.Errorf("%w; failed to stop database process without a password: %v", err, stopErr)
			}
		}
	}
	if err := waitForPortClosed(instance.Port); err != nil {
		return err
	}
	process.IsRunning("database")
	return nil
}

func createMySQLDatabaseByName(instance Instance, name, adminPassword string) error {
	if err := validateMySQLSetup(instance.Engine, name, instance.User); err != nil {
		return err
	}
	sql := fmt.Sprintf(
		"CREATE DATABASE %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n"+
			"GRANT ALL PRIVILEGES ON %s.* TO '%s'@'localhost';\n"+
			"GRANT ALL PRIVILEGES ON %s.* TO '%s'@'127.0.0.1';\n"+
			"FLUSH PRIVILEGES;\n",
		quoteMySQLIdentifier(name), quoteMySQLIdentifier(name), instance.User,
		quoteMySQLIdentifier(name), instance.User,
	)
	client, err := mysqlClient(instance)
	if err != nil {
		return err
	}
	cmd := exec.Command(client, mysqlNamedPipeArgs("--user=root")...)
	cmd.Env = environmentWith("MYSQL_PWD", adminPassword)
	cmd.Stdin = strings.NewReader(sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return mysqlOperationError(instance.Engine, "creating", name, err, string(output), adminPassword)
	}
	return nil
}

func deleteMySQLDatabase(instance Instance, name, adminPassword string) error {
	if err := validateMySQLSetup(instance.Engine, name, instance.User); err != nil {
		return err
	}
	switch strings.ToLower(name) {
	case "mysql", "information_schema", "performance_schema", "sys":
		return fmt.Errorf("database %s cannot be deleted", name)
	}
	client, err := mysqlClient(instance)
	if err != nil {
		return err
	}
	cmd := exec.Command(client, mysqlNamedPipeArgs("--user=root")...)
	cmd.Env = environmentWith("MYSQL_PWD", adminPassword)
	cmd.Stdin = strings.NewReader("DROP DATABASE " + quoteMySQLIdentifier(name) + ";\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return mysqlOperationError(instance.Engine, "deleting", name, err, string(output), adminPassword)
	}
	return nil
}

func validateMySQLSetup(engine Engine, name, user string) error {
	if err := validateSetupIdentifiers(name, user); err != nil {
		return err
	}
	switch strings.ToLower(name) {
	case "mysql", "information_schema", "performance_schema", "sys":
		return fmt.Errorf("database name %s is already in use by the server", name)
	}
	switch strings.ToLower(user) {
	case "mysql.session", "mysql.sys":
		return fmt.Errorf("username %s is already in use by the server", user)
	case "mariadb.sys":
		if engine == MariaDB {
			return fmt.Errorf("username %s is already in use by the server", user)
		}
	}
	return nil
}

func listMySQLDatabases(instance Instance, adminPassword string) ([]DatabaseInfo, error) {
	client, err := mysqlClient(instance)
	if err != nil {
		return nil, err
	}
	query := "SELECT s.SCHEMA_NAME, COALESCE(GROUP_CONCAT(DISTINCT TRIM(BOTH '\\'' FROM SUBSTRING_INDEX(p.GRANTEE, '@', 1)) SEPARATOR ', '), '-') " +
		"FROM information_schema.SCHEMATA s LEFT JOIN information_schema.SCHEMA_PRIVILEGES p " +
		"ON p.TABLE_SCHEMA = s.SCHEMA_NAME " +
		"WHERE s.SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') " +
		"GROUP BY s.SCHEMA_NAME ORDER BY s.SCHEMA_NAME"
	cmd := exec.Command(client, mysqlNamedPipeArgs("--user=root", "--batch", "--skip-column-names", "--execute", query)...)
	cmd.Env = environmentWith("MYSQL_PWD", adminPassword)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, mysqlListError(instance.Engine, err, string(output), adminPassword)
	}
	return parseDatabaseList(string(output), instance.Port)
}

func mysqlClient(instance Instance) (string, error) {
	installDir := databaseInstallDir(instance)
	name := "mysql.exe"
	if instance.Engine == MariaDB {
		name = "mariadb.exe"
	}
	client, err := findExecutable(installDir, name)
	if err != nil && instance.Engine == MariaDB {
		return findExecutable(installDir, "mysql.exe")
	}
	return client, err
}

func mysqlOperationError(engine Engine, action, name string, err error, output, password string) error {
	details := redactDatabasePassword(output, password)
	if strings.Contains(details, "ERROR 1044") {
		return fmt.Errorf("failed %s database %s: admin account %s lacks the required privileges: %w: %s", action, name, engine, err, details)
	}
	if strings.Contains(strings.ToLower(details), "pipe") {
		return fmt.Errorf("failed %s database %s: named pipe %s is not active. Stop the old instance, then run `gacor database start`: %w: %s", action, name, engine, err, details)
	}
	return fmt.Errorf("failed %s database %s: %w: %s", action, name, err, details)
}

func mysqlListError(engine Engine, err error, output, password string) error {
	details := redactDatabasePassword(output, password)
	if strings.Contains(strings.ToLower(details), "pipe") {
		return fmt.Errorf("named pipe %s is unavailable; stop the old instance, then run `gacor database start`: %w: %s", engine, err, details)
	}
	return fmt.Errorf("failed to list databases: %w: %s", err, details)
}

func initializeMySQL(ctx context.Context, serverPath, installDir, dataDir string) error {
	cmd := exec.CommandContext(ctx, serverPath,
		"--initialize-insecure",
		"--basedir="+installDir,
		"--datadir="+dataDir,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to initialize MySQL: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func writeMySQLConfig(baseDir, dataDir string, instance Instance) error {
	path := configPath(instance.Engine, instance.Version)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf("[mysqld]\nbasedir=%s\ndatadir=%s\nport=%d\nbind-address=127.0.0.1\nenable-named-pipe\nsocket=MySQL\nlog-error=%s\n",
		quoteOptionPath(baseDir),
		quoteOptionPath(dataDir),
		instance.Port,
		quoteOptionPath(filepath.Join(dataDir, "server.log")),
	)
	return os.WriteFile(path, []byte(content), 0600)
}

func quoteOptionPath(value string) string {
	value = strings.ReplaceAll(value, `\`, `/`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func startMySQLServer(serverPath, installDir string, instance Instance) error {
	args := []string{
		"--defaults-file=" + configPath(instance.Engine, instance.Version),
		"--enable-named-pipe",
	}
	if err := process.StartBackground("database", serverPath, args, installDir, nil); err != nil {
		return err
	}
	return nil
}

func createMySQLDatabase(ctx context.Context, installDir string, instance Instance, password string) error {
	clientName := "mysql.exe"
	if instance.Engine == MariaDB {
		clientName = "mariadb.exe"
	}
	client, err := findExecutable(installDir, clientName)
	if err != nil {
		return err
	}
	database := quoteMySQLIdentifier(instance.Name)
	username := strings.ReplaceAll(instance.User, "'", "''")
	escapedPassword := strings.ReplaceAll(password, `\`, `\\`)
	escapedPassword = strings.ReplaceAll(escapedPassword, "'", "''")
	sql := mysqlSetupSQL(instance, database, username, escapedPassword)
	args := []string{"--protocol=TCP", "--host=127.0.0.1", fmt.Sprintf("--port=%d", instance.Port), "--user=root"}
	cmd := exec.CommandContext(ctx, client, args...)
	cmd.Stdin = strings.NewReader(sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return mysqlCreateError(instance.Name, err, string(output), password)
	}
	return nil
}

func mysqlSetupSQL(instance Instance, database, username, escapedPassword string) string {
	userStatements := fmt.Sprintf(
		"CREATE USER '%s'@'localhost' IDENTIFIED BY '%s';\nCREATE USER '%s'@'127.0.0.1' IDENTIFIED BY '%s';\n",
		username, escapedPassword, username, escapedPassword,
	)
	adminStatements := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS 'root'@'127.0.0.1' IDENTIFIED BY '%s';\n"+
			"ALTER USER 'root'@'127.0.0.1' IDENTIFIED BY '%s';\n"+
			"GRANT CREATE ON *.* TO 'root'@'127.0.0.1';\n",
		escapedPassword, escapedPassword,
	)
	if strings.EqualFold(instance.User, "root") {
		userStatements = fmt.Sprintf(
			"CREATE USER IF NOT EXISTS 'root'@'127.0.0.1' IDENTIFIED BY '%s';\nALTER USER 'root'@'127.0.0.1' IDENTIFIED BY '%s';\n",
			escapedPassword, escapedPassword,
		)
		adminStatements = "GRANT CREATE ON *.* TO 'root'@'127.0.0.1';\n"
	}
	sql := fmt.Sprintf(
		"ALTER USER 'root'@'localhost' IDENTIFIED BY '%s';\nCREATE DATABASE %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n%s%sGRANT ALL PRIVILEGES ON %s.* TO '%s'@'localhost';\nGRANT ALL PRIVILEGES ON %s.* TO '%s'@'127.0.0.1';\nFLUSH PRIVILEGES;\n",
		escapedPassword, database, userStatements, adminStatements, database, username, database, username,
	)
	return sql
}

func mysqlCreateError(name string, err error, output, password string) error {
	details := strings.TrimSpace(output)
	if password != "" {
		details = strings.ReplaceAll(details, password, "[redacted]")
	}
	if strings.Contains(details, "ERROR 1044") {
		return fmt.Errorf("failed to create database %s: admin account root@127.0.0.1 lacks the global CREATE privilege. This instance was created with an older configuration; the account requires CREATE ON *.*: %w: %s",
			name, err, details)
	}
	return fmt.Errorf("failed to create MySQL database/user: %w: %s", err, details)
}

func mysqlNamedPipeArgs(args ...string) []string {
	return append([]string{"--protocol=PIPE", "--socket=MySQL"}, args...)
}

func quoteMySQLIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func runMySQLAdmin(installDir string, instance Instance, username, password, action string) error {
	clientName := "mysqladmin.exe"
	if instance.Engine == MariaDB {
		clientName = "mariadb-admin.exe"
	}
	admin, err := findExecutable(installDir, clientName)
	if err != nil && instance.Engine == MariaDB {
		admin, err = findExecutable(installDir, "mysqladmin.exe")
	}
	if err != nil {
		return err
	}
	pipeCmd := exec.Command(admin, mysqlNamedPipeArgs("--user="+username, action)...)
	pipeCmd.Env = environmentWith("MYSQL_PWD", password)
	output, pipeErr := pipeCmd.CombinedOutput()
	if pipeErr == nil {
		return nil
	}

	tcpCmd := exec.Command(admin, "--protocol=TCP", "--host=127.0.0.1",
		fmt.Sprintf("--port=%d", instance.Port), "--user="+username, action)
	tcpCmd.Env = environmentWith("MYSQL_PWD", password)
	tcpOutput, tcpErr := tcpCmd.CombinedOutput()
	if tcpErr == nil {
		return nil
	}

	details := strings.TrimSpace(string(tcpOutput))
	if password != "" {
		details = strings.ReplaceAll(details, password, "[redacted]")
	}
	return fmt.Errorf("failed to run %s for %s: named pipe: %v: %s; TCP: %w: %s",
		action, instance.Engine, pipeErr, strings.TrimSpace(string(output)), tcpErr, details)
}

func fetchMySQL() ([]Release, error) {
	const pageURL = "https://dev.mysql.com/downloads/mysql/"
	body, err := fetchText(pageURL)
	if err != nil {
		return fetchMySQLRepositories(err)
	}

	re := regexp.MustCompile(`(?i)(?:https://(?:dev|cdn)\.mysql\.com/(?:get/)?Downloads/MySQL-[^/"'<> ]+/)?mysql-(\d+\.\d+\.\d+)-winx64\.zip`)
	seen := map[string]bool{}
	var releases []Release
	for _, match := range re.FindAllStringSubmatch(body, -1) {
		version := match[1]
		if seen[version] {
			continue
		}
		seen[version] = true
		releases = append(releases, Release{
			Version: version,
			URL:     "https://dev.mysql.com/get/Downloads/MySQL-" + version[:strings.LastIndex(version, ".")] + "/mysql-" + version + "-winx64.zip",
		})
	}
	if len(releases) == 0 {
		return fetchMySQLRepositories(fmt.Errorf("no Windows x64 archive found at %s", pageURL))
	}
	return latestPerSeries(releases), nil
}

func fetchMySQLRepositories(pageErr error) ([]Release, error) {
	tracks := []string{
		"mysql-innovation-community",
		"mysql-8.4-community",
		"mysql-8.0-community",
	}
	seen := map[string]bool{}
	var releases []Release
	var trackErrors []string
	for _, track := range tracks {
		found, err := fetchMySQLRepositoryTrack(track)
		if err != nil {
			trackErrors = append(trackErrors, track+": "+err.Error())
			continue
		}
		for _, release := range found {
			if seen[release.Version] {
				continue
			}
			seen[release.Version] = true
			releases = append(releases, release)
		}
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("unable to fetch MySQL versions from the download page (%v) or the official repository: %s",
			pageErr, strings.Join(trackErrors, "; "))
	}
	if len(trackErrors) > 0 {
		return nil, fmt.Errorf("failed to fetch all official MySQL release tracks: %s", strings.Join(trackErrors, "; "))
	}
	return latestPerSeries(releases), nil
}

func fetchMySQLRepositoryTrack(track string) ([]Release, error) {
	baseURL := "https://repo.mysql.com/yum/" + track + "/el/9/x86_64/"
	var metadata struct {
		Data []struct {
			Type     string `xml:"type,attr"`
			Location struct {
				Href string `xml:"href,attr"`
			} `xml:"location"`
		} `xml:"data"`
	}
	if err := fetchXML(baseURL+"repodata/repomd.xml", &metadata); err != nil {
		return nil, err
	}
	primaryPath := ""
	for _, entry := range metadata.Data {
		if entry.Type == "primary" {
			primaryPath = entry.Location.Href
			break
		}
	}
	if primaryPath == "" {
		return nil, fmt.Errorf("repository metadata has no primary index")
	}
	compressed, err := fetchBytes(baseURL + primaryPath)
	if err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("repository gzip metadata is invalid: %w", err)
	}
	defer reader.Close()

	var packages struct {
		Packages []struct {
			Name    string `xml:"name"`
			Arch    string `xml:"arch"`
			Version struct {
				Value string `xml:"ver,attr"`
			} `xml:"version"`
		} `xml:"package"`
	}
	if err := xml.NewDecoder(reader).Decode(&packages); err != nil {
		return nil, fmt.Errorf("MySQL package metadata is invalid: %w", err)
	}

	seen := map[string]bool{}
	var releases []Release
	for _, pkg := range packages.Packages {
		if pkg.Name != "mysql-community-server" || pkg.Arch != "x86_64" || pkg.Version.Value == "" || seen[pkg.Version.Value] {
			continue
		}
		version := pkg.Version.Value
		parts := strings.Split(version, ".")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid repository version: %s", version)
		}
		directory := strings.Join(parts[:2], ".")
		archiveURL := "https://cdn.mysql.com/Downloads/MySQL-" + directory + "/mysql-" + version + "-winx64.zip"
		ok, err := archiveAvailable(archiveURL)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		seen[version] = true
		releases = append(releases, Release{Version: version, URL: archiveURL})
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("repository does not list an available Windows x64 server archive")
	}
	return latestPerSeries(releases), nil
}
