//go:build !windows

package engine

import "os/exec"

func startManagedProcess(cmd *exec.Cmd) (func() error, func() error, error) {
	configureProcess(cmd)
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return func() error { return killProcessGroup(cmd) }, func() error { return nil }, nil
}

func encodeMI(command string) ([]byte, error) { return []byte(command), nil }
