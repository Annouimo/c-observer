package engine

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

var (
	kernel32Job   = syscall.NewLazyDLL("kernel32.dll")
	createJob     = kernel32Job.NewProc("CreateJobObjectW")
	assignJob     = kernel32Job.NewProc("AssignProcessToJobObject")
	terminateJob  = kernel32Job.NewProc("TerminateJobObject")
	resumeProcess = syscall.NewLazyDLL("ntdll.dll").NewProc("NtResumeProcess")
)

func startManagedProcess(cmd *exec.Cmd) (stop func() error, release func() error, err error) {
	if err := resumeProcess.Find(); err != nil {
		return nil, nil, err
	}
	job, _, callErr := createJob.Call(0, 0)
	if job == 0 {
		return nil, nil, fmt.Errorf("create Windows job: %w", callErr)
	}
	closeJob := func() error { return syscall.CloseHandle(syscall.Handle(job)) }
	stop = func() error {
		ok, _, err := terminateJob.Call(job, 1)
		if ok == 0 {
			return fmt.Errorf("terminate Windows job: %w", err)
		}
		return nil
	}
	// Keep the first thread suspended until the job owns every future descendant.
	const createSuspended = 0x4
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createSuspended}
	if cmd.Cancel != nil {
		cmd.Cancel = stop
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, errors.Join(err, closeJob())
	}
	failed := func(err error) (func() error, func() error, error) {
		killErr := cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, nil, errors.Join(err, killErr, closeJob())
	}
	const processSetQuota = 0x0100
	const processSuspendResume = 0x0800
	handle, err := syscall.OpenProcess(
		processSetQuota|syscall.PROCESS_TERMINATE|processSuspendResume, false, uint32(cmd.Process.Pid))
	if err != nil {
		return failed(fmt.Errorf("open suspended process: %w", err))
	}
	defer syscall.CloseHandle(handle)
	if ok, _, err := assignJob.Call(job, uintptr(handle)); ok == 0 {
		return failed(fmt.Errorf("assign suspended process to Windows job: %w", err))
	}
	if status, _, _ := resumeProcess.Call(uintptr(handle)); uint32(status) != 0 {
		return failed(fmt.Errorf("resume suspended process: NTSTATUS %#x", uint32(status)))
	}
	release = func() error { return errors.Join(stop(), closeJob()) }
	return stop, release, nil
}
