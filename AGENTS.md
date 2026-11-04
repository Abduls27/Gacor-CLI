# Project Context for AI Agents

## Project

GACOR is a Windows-focused Laravel local-development environment manager written
in Go. It installs and manages PHP, Nginx, and one selected local database
engine (MySQL, MariaDB, or PostgreSQL), discovers Laravel projects, and
generates local `.test` virtual hosts.

User-facing setup and command examples are documented in [README.md](README.md).

## Architecture

- `main.go` delegates to `cmd.Execute()`.
- `cmd/` defines Cobra commands and coordinates configuration, services, and UI.
  The root command opens the interactive command selector when invoked without
  a subcommand.
- `internal/config/` loads and saves `gacor.json`. `GACOR_HOME` overrides the
  default `~/.gacor` data directory; installed binaries and runtime data live
  below that directory.
- `internal/ui/` contains Bubble Tea/Lip Gloss terminal screens: home
  navigation, selectors, prompts, progress, and the database manager.
- `internal/process/` starts hidden background processes, tracks their PIDs,
  stops PHP/Nginx, and handles Windows elevation.
- `internal/php/`, `internal/nginx/`, and `internal/database/` manage each
  runtime. Database engine behavior is implemented by `Driver` implementations
  in `internal/database/mysql.go`, `mariadb.go`, and `postgresql.go`; the
  registry and stable engine lookup live in `internal/database/driver.go`.
  `internal/database/database.go` contains shared archive, path, process, and
  validation helpers, while `fetch.go` contains shared release-feed utilities.
  `internal/project/` discovers Laravel applications;
  `internal/hosts/` updates the Windows hosts file; `internal/ssl/` manages
  local certificates.

## Important Behavior

- `gacor run` starts the configured database (if any), PHP FastCGI on
  `127.0.0.1:9000`, and Nginx after generating configuration for discovered
  Laravel projects.
- `gacor stop` stops the configured database as well as PHP and Nginx. Database
  shutdown may prompt for the MySQL/MariaDB administrator password; Windows
  elevation is used for operations that need it, such as updating hosts or
  stopping web-server processes.
- Configuration stores one active database engine, version, port, and initial
  database/user. That server can contain multiple databases. The interactive
  database manager is selected through Database → Create; the direct form is
  `gacor database create <name>`.
- MySQL/MariaDB database administration uses the local named pipe configured
  by GACOR. When changing server startup/configuration behavior, keep the
  installer initialization path and later `database start` path consistent.
- Database passwords are intentionally not persisted in GACOR configuration.
  Do not add password logging or include secrets in errors.
- The local database servers bind to loopback for development. Avoid broadening
  network exposure unless explicitly requested.
- Windows process behavior is implemented in `internal/process/*_windows.go`.
  Preserve hidden-window/background behavior when changing process creation.

## Development and Validation

- Go module: `gacor`; Go version declared in `go.mod`.
- Format changed Go files with `gofmt`.
- Run all tests with `go test ./...`.
- Build all packages with `go build ./...`.
- Database engine behavior and SQL construction live beside their driver
  implementations in `internal/database/`; registry and shared parsing tests
  are in `internal/database/driver_test.go`.
- Prefer focused tests for behavior changes, then run the full suite when
  practical. Do not require a live database for ordinary unit tests.

## Change Guidance

- Keep Cobra command handling in `cmd/`; keep reusable engine/process behavior
  in the relevant `internal/` package.
- To add a database engine, define its `Engine` constant, implement every
  `Driver` method in a dedicated engine file, then register the driver in
  `internal/database/driver.go`. Keep engine-specific release discovery beside
  that driver; put only truly shared helpers in `database.go` or `fetch.go`.
- Preserve the existing Indonesian user-facing language and Bubble Tea UI
  patterns.
- Validate database identifiers before generating SQL, and quote identifiers
  using the engine-specific helpers rather than concatenating unchecked input.
- Treat configuration and runtime data as user data. Do not delete or reset
  them as part of normal code changes.
