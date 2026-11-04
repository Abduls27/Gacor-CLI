package php

import (
	"fmt"
	"os/exec"
)

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
