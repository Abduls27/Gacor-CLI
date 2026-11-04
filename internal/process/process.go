package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gacor/internal/config"
)

func pidFile(name string) string {
	return filepath.Join(config.BaseDir(), name+".pid")
}

func StartBackground(name, exe string, args []string, workDir string, env []string) error {
	cmd := exec.Command(exe, args...)
	cmd.Dir = workDir

	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gagal start %s: %w", name, err)
	}

	if err := os.WriteFile(pidFile(name), []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		out, stopErr := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).CombinedOutput()
		if stopErr != nil {
			return fmt.Errorf("gagal menyimpan PID %s: %w; gagal menghentikan proses: %v: %s", name, err, stopErr, strings.TrimSpace(string(out)))
		}
		return fmt.Errorf("gagal menyimpan PID %s: %w; proses yang baru dijalankan sudah dihentikan", name, err)
	}
	return nil
}

func Stop(name string) error {
	data, err := os.ReadFile(pidFile(name))
	if err != nil {
		return fmt.Errorf("%s tidak sedang berjalan", name)
	}

	pid := strings.TrimSpace(string(data))
	out, err := exec.Command("taskkill", "/F", "/T", "/PID", pid).CombinedOutput()
	if err != nil {
		pidNumber, parseErr := strconv.Atoi(pid)
		if parseErr != nil {
			return fmt.Errorf("PID %s tidak valid untuk %s: %w", pid, name, parseErr)
		}
		running, checkErr := processIsRunning(pidNumber)
		if checkErr != nil {
			return fmt.Errorf("gagal menghentikan %s: %w: %s; gagal memeriksa status proses: %v", name, err, strings.TrimSpace(string(out)), checkErr)
		}
		if !running {
			if removeErr := os.Remove(pidFile(name)); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("gagal menghapus PID %s: %w", name, removeErr)
			}
			return nil
		}
		return fmt.Errorf("gagal menghentikan %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	if err := os.Remove(pidFile(name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("gagal menghapus PID %s: %w", name, err)
	}
	return nil
}

func IsRunning(name string) (bool, int) {
	data, err := os.ReadFile(pidFile(name))
	if err != nil {
		return false, 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, 0
	}

	running, err := processIsRunning(pid)
	if err != nil {
		return false, 0
	}

	if running {
		return true, pid
	}

	os.Remove(pidFile(name))
	return false, 0
}

func processIsRunning(pid int) (bool, error) {
	out, err := exec.Command("tasklist", "/FI",
		fmt.Sprintf("PID eq %d", pid), "/NH").Output()
	if err != nil {
		return false, err
	}
	for _, field := range strings.Fields(string(out)) {
		if field == strconv.Itoa(pid) {
			return true, nil
		}
	}
	return false, nil
}
