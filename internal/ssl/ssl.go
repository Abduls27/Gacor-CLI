package ssl

import (
	"fmt"
	"gacor/internal/config"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/schollz/progressbar/v3"
)

const mkcertURL = "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-windows-amd64.exe"

func MkcertPath() string {
	return filepath.Join(config.BinDir(), "mkcert.exe")
}

func CertDir() string {
	return filepath.Join(config.BaseDir(), "certs")
}

func CertPath(domain string) string {
	return filepath.Join(CertDir(), domain+".pem")
}

func KeyPath(domain string) string {
	return filepath.Join(CertDir(), domain+"-key.pem")
}

func IsInstalled() bool {
	_, err := os.Stat(MkcertPath())
	return err == nil
}

func Download() error {
	fmt.Println("Downloading mkcert...")
	dest := MkcertPath()
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	resp, err := http.Get(mkcertURL)
	if err != nil {
		return fmt.Errorf("failed to download mkcert: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	bar := progressbar.DefaultBytes(resp.ContentLength, "downloading")
	_, err = io.Copy(io.MultiWriter(f, bar), resp.Body)
	return err
}

func InstallCA() error {
	cmd := exec.Command(MkcertPath(), "-install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func GenerateCert(domains []string) error {
	if err := os.MkdirAll(CertDir(), 0755); err != nil {
		return err
	}

	for _, domain := range domains {
		fmt.Printf("Generating certificate for %s...\n", domain)
		args := []string{
			"-cert-file", CertPath(domain),
			"-key-file", KeyPath(domain),
			domain,
		}
		cmd := exec.Command(MkcertPath(), args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to generate certificate %s: %w", domain, err)
		}
	}
	return nil
}

func CertExists(domain string) bool {
	_, err := os.Stat(CertPath(domain))
	return err == nil
}
