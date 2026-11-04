package php

import (
	"fmt"
	"gacor/internal/config"
	"gacor/internal/download"
	"os"
	"path/filepath"
	"strings"

	"regexp"
)

func IsInstalled(version string) bool {
	dir := filepath.Join(config.PHPDir(), "php-"+version)
	exe := filepath.Join(dir, "php.exe")
	_, err := os.Stat(exe)
	return err == nil
}

func ExePath(version string) string {
	return filepath.Join(config.PHPDir(), "php-"+version, "php.exe")
}

func DownloadFromURL(version, url string, cfg *config.Config, onEvent func(stage, label string, done, total int64)) error {
	destDir := filepath.Join(config.PHPDir(), "php-"+version)
	zipPath := filepath.Join(config.PHPDir(), "php-"+version+".zip")

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	if err := downloadFileWithProgress(zipPath, url, "PHP "+version, onEvent); err != nil {
		return fmt.Errorf("download gagal: %w", err)
	}

	onEvent("extract", "PHP "+version, 0, 0)
	if err := unzip(zipPath, destDir); err != nil {
		return fmt.Errorf("extract gagal: %w", err)
	}
	os.Remove(zipPath)

	onEvent("configure", "PHP "+version, 0, 0)
	if err := ConfigurePHPIni(version, cfg); err != nil {
		fmt.Printf("Warning: gagal setup php.ini: %s\n", err)
	}

	return nil
}

func downloadFileWithProgress(dest, url, label string, onEvent func(stage, label string, done, total int64)) error {
	return download.FileWithProgress(dest, url, label, func(label string, done, total int64) {
		if onEvent != nil {
			onEvent("download", label, done, total)
		}
	})
}

func unzip(src, dest string) error {
	// Uses PowerShell on Windows for extraction
	// This is called via process package
	_ = src
	_ = dest
	return unzipWindows(src, dest)
}

func CGIPath(version string) string {
	return filepath.Join(config.PHPDir(), "php-"+version, "php-cgi.exe")
}

func ConfigurePHPIni(version string, cfg *config.Config) error {
	phpDir := filepath.Join(config.PHPDir(), "php-"+version)
	devIni := filepath.Join(phpDir, "php.ini-development")
	iniPath := filepath.Join(phpDir, "php.ini")

	// Kalau php.ini sudah ada, skip (jangan timpa setting manual user)
	if _, err := os.Stat(iniPath); err == nil {
		return nil
	}

	data, err := os.ReadFile(devIni)
	if err != nil {
		return fmt.Errorf("gagal baca php.ini-development: %w", err)
	}
	content := string(data)

	extDir := filepath.ToSlash(filepath.Join(phpDir, "ext"))

	replacements := map[string]string{
		`;extension_dir = "ext"`: fmt.Sprintf(`extension_dir = "%s"`, extDir),
		`;extension=curl`:        `extension=curl`,
		`;extension=fileinfo`:    `extension=fileinfo`,
		`;extension=mbstring`:    `extension=mbstring`,
		`;extension=openssl`:     `extension=openssl`,
		`;extension=pdo_mysql`:   `extension=pdo_mysql`,
		`;extension=pdo_sqlite`:  `extension=pdo_sqlite`,
		`;extension=sqlite3`:     `extension=sqlite3`,
		`;extension=zip`:         `extension=zip`,
		`;extension=gd`:          `extension=gd`,
		`;extension=intl`:        `extension=intl`,
		`;extension=pdo_pgsql`:   `extension=pdo_pgsql`,
		`;extension=pgsql`:       `extension=pgsql`,
		`;extension=exif`:        `extension=exif`,
		`;extension=bcmath`:      `extension=bcmath`,
		`;extension=redis`:       `extension=redis`,
	}

	for old, new := range replacements {
		content = strings.Replace(content, old, new, 1)
	}

	// Setting tambahan ala Herd — upload besar, memory cukup, timezone, error display
	extra := fmt.Sprintf(`
; --- gacor defaults ---
upload_max_filesize = %s
post_max_size = %s
max_execution_time = 300
memory_limit = %s
max_input_time = 300
max_file_uploads = 50
date.timezone = %s
display_errors = On
error_reporting = E_ALL & ~E_DEPRECATED & ~E_STRICT

zend_extension=opcache

[opcache]
opcache.enable=1
opcache.enable_cli=0
opcache.memory_consumption=64
opcache.interned_strings_buffer=8
opcache.max_accelerated_files=10000
opcache.max_wasted_percentage=5
opcache.use_cwd=1
`,
		cfg.PHPUploadMax,
		cfg.PHPUploadMax,
		cfg.PHPMemoryLimit,
		cfg.PHPTimezone,
	)

	content += extra

	return os.WriteFile(iniPath, []byte(content), 0644)
}

// InstalledVersions mengembalikan semua versi PHP yang ada di folder bin,
// misal ["8.3.14", "8.4.26"]
func InstalledVersions() ([]string, error) {
	entries, err := os.ReadDir(config.PHPDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	re := regexp.MustCompile(`^php-(\d+\.\d+\.\d+)$`)
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m := re.FindStringSubmatch(e.Name())
		if len(m) == 2 {
			out = append(out, m[1])
		}
	}
	return out, nil
}

// RemoveVersion menghapus folder instalasi PHP versi tertentu
func RemoveVersion(version string) error {
	dir := filepath.Join(config.PHPDir(), "php-"+version)
	return os.RemoveAll(dir)
}

