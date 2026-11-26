//go:build darwin || linux

package engine

import "syscall"

func verifyInferiorExited(pid int) error {
	err := syscall.Kill(pid, 0)
	if err == syscall.ESRCH {
		return nil
	}
	if err == nil {
		return syscall.EBUSY
	}
	return err
}
