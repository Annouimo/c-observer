package engine

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func verifyInferiorExited(pid int) error {
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, syscall.Errno(87)) {
		return nil // Windows reports ERROR_INVALID_PARAMETER for an absent PID.
	}
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)
	state, err := syscall.WaitForSingleObject(handle, 0)
	if err != nil {
		return err
	}
	if state != syscall.WAIT_OBJECT_0 {
		return errors.New("inferior is still running")
	}
	return nil
}

func TestWindowsManagedProcessStopsDescendants(t *testing.T) {
	const helper = "C_OBSERVER_JOB_TEST_HELPER"
	switch os.Getenv(helper) {
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=^TestWindowsManagedProcessStopsDescendants$")
		child.Env = append(os.Environ(), helper+"=child")
		_, releaseChild, err := startManagedProcess(child)
		if err != nil {
			t.Fatal(err)
		}
		defer releaseChild()
		if err := os.WriteFile(os.Getenv("C_OBSERVER_JOB_TEST_PID"), []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
			t.Fatal(err)
		}
		_ = child.Wait()
		return
	case "child":
		time.Sleep(30 * time.Second)
		return
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd := exec.Command(os.Args[0], "-test.run=^TestWindowsManagedProcessStopsDescendants$")
	cmd.Env = append(os.Environ(), helper+"=parent", "C_OBSERVER_JOB_TEST_PID="+pidFile)
	stop, release, err := startManagedProcess(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	parent, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(parent)
	var pid int
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		data, _ := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(string(data))
		if pid != 0 {
			break
		}
	}
	if pid == 0 {
		t.Fatal("managed process did not create a child")
	}
	child, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(child)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	for role, handle := range map[string]syscall.Handle{"parent": parent, "child": child} {
		state, err := syscall.WaitForSingleObject(handle, 2000)
		if err != nil || state != syscall.WAIT_OBJECT_0 {
			t.Errorf("%s survived forced job cleanup: wait=%d err=%v", role, state, err)
		}
	}
}
