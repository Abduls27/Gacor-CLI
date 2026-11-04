package nginx

import (
	"fmt"
	"gacor/internal/config"
	"gacor/internal/download"
	"os"
	"os/exec"
	"path/filepath"
)

func DownloadFromURL(version, url string, onEvent func(stage, label string, done, total int64)) error {
	destDir := filepath.Join(config.NginxDir(), "nginx-"+version)
	zipPath := filepath.Join(config.NginxDir(), "nginx-"+version+".zip")
	label := "Nginx " + version

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	if err := downloadFileWithProgress(zipPath, url, label, onEvent); err != nil {
		return fmt.Errorf("download gagal: %w", err)
	}

	onEvent("extract", label, 0, 0)
	if err := unzipWindows(zipPath, config.NginxDir()); err != nil {
		return fmt.Errorf("extract gagal: %w", err)
	}

	os.Remove(zipPath)
	return nil
}

func downloadFileWithProgress(dest, url, label string, onEvent func(stage, label string, done, total int64)) error {
	return download.FileWithProgress(dest, url, label, func(label string, done, total int64) {
		if onEvent != nil {
			onEvent("download", label, done, total)
		}
	})
}

func unzipWindows(src, dest string) error {
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf(`Expand-Archive -Path '%s' -DestinationPath '%s' -Force`, src, dest),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
