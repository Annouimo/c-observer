//go:build !darwin && !linux

package engine

import (
	"os/exec"
)

func configureProcess(_ *exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
