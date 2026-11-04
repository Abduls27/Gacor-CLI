package database

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"gacor/internal/config"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Instance struct {
	Engine  Engine
	Version string
	Name    string
	User    string
	Port    int
}

type DatabaseInfo struct {
	Name string
	User string
	Port int
}

var identifierRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

func ValidIdentifier(value string) bool {
	return identifierRE.MatchString(value)
}

func validateSetupIdentifiers(name, user string) error {
	if !ValidIdentifier(name) || !ValidIdentifier(user) {
		return fmt.Errorf("nama database dan user hanya boleh berisi huruf, angka, dan underscore; karakter pertama harus huruf atau underscore")
	}
	return nil
}

func PortInUse(port int) bool {
	return portInUse(port)
}

func InstallArchive(ctx context.Context, release Release, engine Engine, onEvent func(stage, label string, done, total int64)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	driver, err := driverFor(engine)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(release.Version) == "" || strings.TrimSpace(release.URL) == "" {
		return "", fmt.Errorf("metadata versi %s tidak lengkap", engine)
	}

	root := filepath.Join(config.BinDir(), "database", string(engine))
	installDir := filepath.Join(root, release.Version)
	if _, err := os.Stat(filepath.Join(installDir, ".gacor-installed")); err == nil {
		return installDir, nil
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}

	archivePath := filepath.Join(root, release.Version+".zip")
	defer os.Remove(archivePath)
	if err := downloadArchive(ctx, archivePath, release.URL, engine.String()+" "+release.Version, onEvent); err != nil {
		return "", err
	}

	if onEvent == nil {
		onEvent = func(string, string, int64, int64) {}
	}
	onEvent("extract", engine.String()+" "+release.Version, 0, 0)
	if err := extractZip(ctx, archivePath, installDir); err != nil {
		return "", fmt.Errorf("gagal extract %s %s: %w", engine, release.Version, err)
	}
	if _, err := driver.FindServer(installDir); err != nil {
		return "", err
	}
	return installDir, nil
}

func IsInstalled(engine Engine, version string) bool {
	_, err := os.Stat(filepath.Join(config.BinDir(), "database", string(engine), version, ".gacor-installed"))
	return err == nil
}

func downloadArchive(ctx context.Context, dest, source, label string, onEvent func(stage, label string, done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "gacor database manager")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("gagal mengunduh %s: %w", label, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gagal mengunduh %s: server merespons HTTP %d", label, resp.StatusCode)
	}
	if onEvent == nil {
		onEvent = func(string, string, int64, int64) {}
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, 64*1024)
	var done int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			written, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
			done += int64(n)
			onEvent("download", label, done, resp.ContentLength)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

func extractZip(ctx context.Context, source, destination string) error {
	archive, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer archive.Close()
	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	root, err := filepath.Abs(destination)
	if err != nil {
		return err
	}

	for _, entry := range archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(entry.Name))
		relative, err := filepath.Rel(root, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("arsip memuat path di luar direktori instalasi: %q", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if entry.Mode()&0111 != 0 {
			mode = 0755
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, contextReader{ctx: ctx, reader: input})
		inputErr, outputErr := input.Close(), output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		if outputErr != nil {
			return outputErr
		}
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func findExecutable(root string, names ...string) (string, error) {
	want := make(map[string]bool, len(names))
	for _, name := range names {
		want[strings.ToLower(name)] = true
	}
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && want[strings.ToLower(entry.Name())] {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("arsip %s tidak memuat executable %s", root, strings.Join(names, " atau "))
	}
	return found, nil
}

func executableDirectory(serverPath string) string {
	return filepath.Dir(filepath.Dir(serverPath))
}

func dataDirectory(engine Engine, version string) string {
	return filepath.Join(config.BaseDir(), "data", "database", string(engine), version)
}

func configPath(engine Engine, version string) string {
	return filepath.Join(config.BaseDir(), "database", string(engine), version+".conf")
}

func databaseInstallDir(instance Instance) string {
	return filepath.Join(config.BinDir(), "database", string(instance.Engine), instance.Version)
}

func markInstalled(instance Instance) error {
	path := filepath.Join(config.BinDir(), "database", string(instance.Engine), instance.Version, ".gacor-installed")
	return os.WriteFile(path, []byte(instance.Version), 0644)
}

func parseDatabaseList(output string, port int) ([]DatabaseInfo, error) {
	var databases []DatabaseInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 || strings.TrimSpace(fields[0]) == "" {
			return nil, fmt.Errorf("format daftar database tidak valid: %q", line)
		}
		user := strings.TrimSpace(fields[1])
		if user == "" {
			user = "-"
		}
		databases = append(databases, DatabaseInfo{
			Name: strings.TrimSpace(fields[0]),
			User: user,
			Port: port,
		})
	}
	return databases, nil
}

func redactDatabasePassword(output, password string) string {
	details := strings.TrimSpace(output)
	if password != "" {
		details = strings.ReplaceAll(details, password, "[redacted]")
	}
	return details
}

func waitForPort(ctx context.Context, port int) error {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return fmt.Errorf("database tidak membuka port %d setelah 45 detik", port)
}

func environmentWith(key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(entry), prefix) {
			env = append(env, entry)
		}
	}
	return append(env, key+"="+value)
}

func portInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func waitForPortClosed(port int) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !portInUse(port) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("database masih menerima koneksi pada port %d setelah perintah stop", port)
}
