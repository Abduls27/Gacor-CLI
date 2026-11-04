package nginx

import (
	"gacor/internal/config"
	"os"
	"path/filepath"
)

func IsInstalled(version string) bool {
	exe := ExePath(version)
	_, err := os.Stat(exe)
	return err == nil
}

func ExePath(version string) string {
	return filepath.Join(config.NginxDir(), "nginx-"+version, "nginx.exe")
}

func ConfPath(version string) string {
	return filepath.Join(config.NginxDir(), "nginx-"+version, "conf", "nginx.conf")
}

// RemoveVersion removes the installation directory for the specified Nginx version.
func RemoveVersion(version string) error {
	dir := filepath.Join(config.NginxDir(), "nginx-"+version)
	return os.RemoveAll(dir)
}
