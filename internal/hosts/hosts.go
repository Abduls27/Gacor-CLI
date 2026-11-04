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

	// Remove all existing GACOR lines.
	lines := strings.Split(string(data), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.Contains(line, marker) {
			cleaned = append(cleaned, line)
		}
	}

	// Trim trailing newlines, then add the new domains.
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
