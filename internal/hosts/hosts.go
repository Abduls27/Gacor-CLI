package hosts

import (
	"os"
	"strings"
)

const hostsPath = `C:\Windows\System32\drivers\etc\hosts`
const marker = "# gacor-managed"

func Update(domains []string) error {
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		return err
	}

	// Hapus semua baris gacor lama
	lines := strings.Split(string(data), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.Contains(line, marker) {
			cleaned = append(cleaned, line)
		}
	}

	// Trim trailing newlines lalu tambah domain baru
	content := strings.TrimRight(strings.Join(cleaned, "\n"), "\n\r")
	content += "\n"
	for _, domain := range domains {
		content += "127.0.0.1  " + domain + "  " + marker + "\n"
	}

	return os.WriteFile(hostsPath, []byte(content), 0644)
}


func NeedsUpdate(domains []string) bool {
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		return true
	}
	content := string(data)
	for _, domain := range domains {
		if !strings.Contains(content, domain+"  "+marker) {
			return true
		}
	}
	return false
}