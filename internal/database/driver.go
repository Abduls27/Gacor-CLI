package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Engine string

const (
	MySQL      Engine = "mysql"
	MariaDB    Engine = "mariadb"
	PostgreSQL Engine = "postgresql"
)

type Release struct {
	Version string
	URL     string
}

func (e Engine) String() string {
	driver, err := driverFor(e)
	if err != nil {
		return string(e)
	}
	return driver.DisplayName()
}

type Driver interface {
	Engine() Engine
	DisplayName() string
	DefaultPort() int
	DataDirectoryMarker() string
	FetchAvailableVersions() ([]Release, error)
	FindServer(installDir string) (string, error)
	ValidateSetup(name, user string) error
	InitializeAndStart(ctx context.Context, instance Instance, password string, onEvent func(stage, label string)) error
	Start(instance Instance) error
	Stop(instance Instance, password string) error
	CreateDatabase(instance Instance, name, adminPassword string) error
	DeleteDatabase(instance Instance, name, adminPassword string) error
	ListDatabases(instance Instance, adminPassword string) ([]DatabaseInfo, error)
	IsRunning(instance Instance) bool
}

var drivers = map[Engine]Driver{
	MySQL:      mysqlDriver{},
	MariaDB:    mariaDBDriver{},
	PostgreSQL: postgreSQLDriver{},
}

func driverFor(engine Engine) (Driver, error) {
	driver, ok := drivers[engine]
	if !ok {
		return nil, fmt.Errorf("database tidak didukung: %s", string(engine))
	}
	return driver, nil
}

func Engines() []Engine {
	engines := make([]Engine, 0, len(drivers))
	for engine := range drivers {
		engines = append(engines, engine)
	}
	sort.Slice(engines, func(i, j int) bool {
		left, right := drivers[engines[i]], drivers[engines[j]]
		if left.DefaultPort() == right.DefaultPort() {
			return strings.Compare(string(engines[i]), string(engines[j])) < 0
		}
		return left.DefaultPort() < right.DefaultPort()
	})
	return engines
}

func DefaultPort(engine Engine) int {
	driver, err := driverFor(engine)
	if err != nil {
		return 0
	}
	return driver.DefaultPort()
}

func ValidateSetup(engine Engine, name, user string) error {
	driver, err := driverFor(engine)
	if err != nil {
		return err
	}
	return driver.ValidateSetup(name, user)
}

func InitializeAndStart(ctx context.Context, instance Instance, password string, onEvent func(stage, label string)) error {
	driver, err := driverFor(instance.Engine)
	if err != nil {
		return err
	}
	return driver.InitializeAndStart(ctx, instance, password, onEvent)
}

func Start(instance Instance) error {
	driver, err := driverFor(instance.Engine)
	if err != nil {
		return err
	}
	return driver.Start(instance)
}

func Stop(instance Instance, password string) error {
	driver, err := driverFor(instance.Engine)
	if err != nil {
		return err
	}
	return driver.Stop(instance, password)
}

func CreateDatabase(instance Instance, name, adminPassword string) error {
	driver, err := driverFor(instance.Engine)
	if err != nil {
		return err
	}
	return driver.CreateDatabase(instance, name, adminPassword)
}

func DeleteDatabase(instance Instance, name, adminPassword string) error {
	driver, err := driverFor(instance.Engine)
	if err != nil {
		return err
	}
	return driver.DeleteDatabase(instance, name, adminPassword)
}

func ListDatabases(instance Instance, adminPassword string) ([]DatabaseInfo, error) {
	driver, err := driverFor(instance.Engine)
	if err != nil {
		return nil, err
	}
	return driver.ListDatabases(instance, adminPassword)
}

func IsRunning(instance Instance) bool {
	driver, err := driverFor(instance.Engine)
	return err == nil && driver.IsRunning(instance)
}

func validateInstanceForInitialization(ctx context.Context, instance Instance) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateSetup(instance.Engine, instance.Name, instance.User); err != nil {
		return err
	}
	if instance.Port < 1 || instance.Port > 65535 {
		return fmt.Errorf("port database tidak valid: %d", instance.Port)
	}
	if portInUse(instance.Port) {
		return fmt.Errorf("port %d sudah digunakan; hentikan aplikasi lain yang memakai port ini sebelum setup", instance.Port)
	}
	return nil
}

func ensureEmptyDataDirectory(instance Instance, dataDir string) error {
	driver, err := driverFor(instance.Engine)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dataDir, driver.DataDirectoryMarker())); err == nil {
		return fmt.Errorf("data %s sudah ada di %s; gunakan database yang sudah diinisialisasi atau pilih versi lain", instance.Engine, dataDir)
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dataDir, 0755)
}

func notifyDatabaseEvent(onEvent func(stage, label string), stage string, instance Instance) {
	if onEvent != nil {
		onEvent(stage, instance.Engine.String()+" "+instance.Version)
	}
}
