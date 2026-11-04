package process

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const createNoWindow = 0x08000000

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
		HideWindow:    true,
	}
}

func checkProcess(proc *os.Process) error {
	out, err := exec.Command("tasklist", "/FI",
		fmt.Sprintf("PID eq %d", proc.Pid), "/NH").Output()
	if err != nil {
		return err
	}
	if strings.Contains(string(out), strconv.Itoa(proc.Pid)) {
		return nil // masih jalan
	}
	return os.ErrProcessDone
}