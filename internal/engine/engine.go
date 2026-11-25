// Package engine runs local C programs under LLDB's debug adapter.
package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Annouimo/c-observer/internal/observer"
)

const cleanupTimeout = 2 * time.Second

// Engine is stateless. Each Run owns its adapter, executable and capture files.
type Engine struct{}

var _ observer.Engine = Engine{}

func (Engine) Run(ctx context.Context, config observer.LaunchConfig) (result observer.RunResult) {
	limits, err := defaults(config.Limits)
	if err != nil {
		result.Reason, result.Message = observer.InvalidConfig, err.Error()
		return
	}
	if err := validateRanges(config.ArrayRanges); err != nil {
		result.Reason, result.Message = observer.InvalidConfig, err.Error()
		return
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(limits.TimeoutMillis)*time.Millisecond)
	defer cancel()
	result.Reason = observer.BackendFailed
	finishError := func(err error) {
		result.Message = err.Error()
		switch {
		case errors.Is(ctx.Err(), context.Canceled):
			result.Reason = observer.Cancelled
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			result.Reason = observer.TimedOut
		case errors.Is(err, context.DeadlineExceeded):
			result.Reason = observer.RequestTimedOut
		}
	}
	if err := ctx.Err(); err != nil {
		finishError(err)
		return
	}
	if config.SourcePath == "" {
		result.Reason, result.Message = observer.InvalidConfig, "C source path is required"
		return
	}
	dir, err := filepath.Abs(config.WorkDir)
	if err != nil {
		finishError(err)
		return
	}
	source := absolute(dir, config.SourcePath)
	files := map[string]string{}
	for _, name := range append([]string{source}, config.VisibleSourcePaths...) {
		path := absolute(dir, name)
		if _, exists := files[path]; exists {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			finishError(err)
			return
		}
		files[path] = string(data)
		result.Session.Sources = append(result.Session.Sources, observer.SourceFile{Path: path, Text: string(data)})
	}
	compiler, err := tool(ctx, dir, config.Compiler, "clang")
	if err != nil {
		finishError(err)
		return
	}
	debugger, err := tool(ctx, dir, config.Debugger, "lldb-dap", "lldb-dap-19")
	if err != nil {
		finishError(err)
		return
	}
	temp, err := os.MkdirTemp("", "c-observer-")
	if err != nil {
		finishError(err)
		return
	}
	defer func() {
		if err := os.RemoveAll(temp); err != nil {
			result.CleanupError = strings.TrimSpace(result.CleanupError + " " + err.Error())
		}
	}()
	exe := filepath.Join(temp, "program")
	args := append(append([]string{"-std=c99"}, config.CompilerArgs...), "-g", "-O0", source, "-o", exe)
	build := exec.CommandContext(ctx, compiler, args...)
	build.Dir = dir
	configureProcess(build)
	build.Cancel = func() error { return killProcessGroup(build) }
	build.WaitDelay = cleanupTimeout
	var diagnostics cappedBuffer
	diagnostics.limit = limits.MaxOutputBytes
	build.Stdout, build.Stderr = &diagnostics, &diagnostics
	if err := build.Run(); err != nil {
		if ctx.Err() == nil {
			result.Reason = observer.CompileFailed
		}
		finishError(err)
		if message := diagnostics.String(); message != "" {
			result.Diagnostics = append(result.Diagnostics, observer.Diagnostic{Severity: "error", Message: message})
		}
		return
	}
	for path, original := range files {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != original {
			finishError(fmt.Errorf("source changed while compiling: %s", path))
			return
		}
	}
	input, output, stderr := filepath.Join(temp, "stdin"), filepath.Join(temp, "stdout"), filepath.Join(temp, "stderr")
	if err := os.WriteFile(input, config.Stdin, 0600); err != nil {
		finishError(err)
		return
	}
	d, err := openAdapter(debugger, dir)
	if err != nil {
		finishError(err)
		return
	}
	result.Session.Backend = "lldb-dap"
	defer func() {
		result.CleanupError = d.close()
		var err error
		result.Output, err = readOutput(output, stderr, limits.MaxOutputBytes)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, observer.Diagnostic{Severity: "error", Message: err.Error()})
		}
	}()

	run := capture{adapter: d, ctx: ctx, limits: limits, dir: dir, files: files,
		config: config, result: &result, output: output, stderr: stderr}
	if err := run.observe(exe, input); err != nil {
		finishError(err)
	}
	return
}

func tool(ctx context.Context, dir, override string, names ...string) (string, error) {
	if override != "" {
		if strings.ContainsRune(override, filepath.Separator) {
			override = absolute(dir, override)
		}
		return exec.LookPath(override)
	}
	if runtime.GOOS == "darwin" {
		output, err := exec.CommandContext(ctx, "xcrun", "--find", names[0]).Output()
		if err == nil {
			return exec.LookPath(strings.TrimSpace(string(output)))
		}
	}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s not found; install or select a toolchain that provides it", names[0])
}

func absolute(dir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(dir, path)
}

func defaults(l observer.Limits) (observer.Limits, error) {
	for ptr, value := range map[*int]int{
		&l.MaxSnapshots: 1000, &l.MaxArrayElements: 16, &l.MaxDepth: 4,
		&l.MaxSnapshotNodes: 512, &l.MaxOutputBytes: 64 << 10,
	} {
		if *ptr < 0 {
			return l, errors.New("capture limits must be nonnegative")
		}
		if *ptr == 0 {
			*ptr = value
		}
	}
	for ptr, value := range map[*int64]int64{
		&l.TimeoutMillis: 10000, &l.RequestTimeoutMillis: 2000, &l.MaxHistoryBytes: 32 << 20,
	} {
		if *ptr < 0 {
			return l, errors.New("capture limits must be nonnegative")
		}
		if *ptr == 0 {
			*ptr = value
		}
	}
	if l.TimeoutMillis > int64((1<<63-1)/time.Millisecond) ||
		l.RequestTimeoutMillis > int64((1<<63-1)/time.Millisecond) {
		return l, errors.New("timeout exceeds supported duration")
	}
	return l, nil
}

type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := min(len(p), b.limit-b.Len())
	_, _ = b.Buffer.Write(p[:n])
	return len(p), nil
}

func readPrefix(path string, limit int) ([]byte, bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)))
	if err != nil {
		return nil, false, err
	}
	info, err := f.Stat()
	return data, err == nil && info.Size() > int64(len(data)), err
}

func readOutput(stdout, stderr string, limit int) (observer.OutputState, error) {
	var output observer.OutputState
	var err, stderrErr error
	output.Stdout, output.StdoutTruncated, err = readPrefix(stdout, limit)
	output.Stderr, output.StderrTruncated, stderrErr = readPrefix(stderr, limit)
	return output, errors.Join(err, stderrErr)
}

type capture struct {
	*adapter
	ctx            context.Context
	limits         observer.Limits
	dir            string
	files          map[string]string
	config         observer.LaunchConfig
	result         *observer.RunResult
	output, stderr string
}

func (c *capture) ask(command string, args map[string]any, body any) error {
	ctx, cancel := context.WithTimeout(c.ctx, time.Duration(c.limits.RequestTimeoutMillis)*time.Millisecond)
	defer cancel()
	if err := c.request(ctx, command, args, body); err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	return nil
}

func (c *capture) wait(names ...string) (dapMessage, error) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Duration(c.limits.RequestTimeoutMillis)*time.Millisecond)
	defer cancel()
	return c.event(ctx, names...)
}

func (c *capture) observe(exe, input string) error {
	var caps struct {
		FunctionBreakpoints bool `json:"supportsFunctionBreakpoints"`
	}
	if err := c.ask("initialize", map[string]any{"adapterID": "lldb", "linesStartAt1": true,
		"columnsStartAt1": true, "supportsVariablePaging": true}, &caps); err != nil {
		return err
	}
	c.result.Session.Capabilities.ExplicitStops = true
	// The adapter interprets this as an LLDB command, not a shell command.
	launch := "process launch --stop-at-entry -i " + lldbQuote(input) +
		" -o " + lldbQuote(c.output) + " -e " + lldbQuote(c.stderr)
	if err := c.ask("launch", map[string]any{"program": exe, "cwd": c.dir, "disableASLR": false,
		"initCommands": []string{"settings set target.disable-aslr false"}, "launchCommands": []string{launch}}, nil); err != nil {
		return err
	}
	if _, err := c.wait("initialized"); err != nil {
		return err
	}
	if !caps.FunctionBreakpoints {
		return errors.New("LLDB does not support a breakpoint at main")
	}
	if err := c.ask("setFunctionBreakpoints", map[string]any{"breakpoints": []map[string]string{{"name": "main"}}}, nil); err != nil {
		return err
	}
	if err := c.keepLocations(); err != nil {
		return err
	}
	if err := c.ask("configurationDone", nil, nil); err != nil {
		return err
	}
	var historyBytes int64
	for {
		event, err := c.wait("stopped", "exited", "terminated")
		if err != nil {
			return err
		}
		if event.Event != "stopped" {
			return c.finishEvent(event)
		}
		var stop struct {
			ThreadID    int    `json:"threadId"`
			Reason      string `json:"reason"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(event.Body, &stop); err != nil {
			return err
		}
		frames, err := c.stack(stop.ThreadID)
		if err != nil {
			return err
		}
		visible := len(frames) > 0 && frames[0].Location.File != "" && !c.skip(frames[0].Location.File)
		crash := stop.Reason == "exception"
		if visible || crash || stop.Reason == "breakpoint" && len(frames) > 0 && c.explicit(frames[0].Location) {
			snapshot, err := c.snapshot(frames)
			if err != nil {
				if crash {
					c.result.Reason, c.result.Message = observer.Signaled, stop.Description
					return nil
				}
				return err
			}
			encoded, err := json.Marshal(snapshot)
			if err != nil {
				return err
			}
			historyBytes += int64(len(encoded))
			if historyBytes > c.limits.MaxHistoryBytes {
				c.result.Reason, c.result.Message = observer.LimitReached, "history byte limit reached"
				return nil
			}
			c.result.Snapshots = append(c.result.Snapshots, snapshot)
		}
		if crash {
			c.result.Reason, c.result.Message = observer.Signaled, stop.Description
			return nil
		}
		if len(c.result.Snapshots) >= c.limits.MaxSnapshots {
			c.result.Reason, c.result.Message = observer.LimitReached, "snapshot limit reached"
			return nil
		}
		command := "continue"
		if visible {
			command = "next"
		}
		if err := c.ask(command, map[string]any{"threadId": stop.ThreadID}, nil); err != nil {
			return err
		}
	}
}

func lldbQuote(value string) string {
	return "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(value) + "\""
}

func (c *capture) finishEvent(e dapMessage) error {
	if e.Event != "exited" {
		return errors.New("LLDB terminated without an inferior exit status")
	}
	var exit struct {
		Code *int `json:"exitCode"`
	}
	if err := json.Unmarshal(e.Body, &exit); err != nil {
		return err
	}
	if exit.Code == nil {
		return errors.New("LLDB did not report an exit code")
	}
	c.result.ExitCode, c.result.Reason = exit.Code, observer.Exited
	if *exit.Code != 0 {
		c.result.Reason = observer.NonzeroExit
	}
	return nil
}

func (d *adapter) close() string {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout/2)
	defer cancel()
	err := d.request(ctx, "disconnect", map[string]bool{"terminateDebuggee": true}, nil)
	close(d.done)
	d.stdin.Close()
	d.stdout.Close()
	go func() { d.exited <- d.cmd.Wait() }()
	select {
	case <-d.exited:
	case <-ctx.Done():
		if killErr := killProcessGroup(d.cmd); killErr != nil {
			return errors.Join(err, killErr).Error()
		}
		select {
		case <-d.exited:
		case <-time.After(cleanupTimeout / 2):
			return "LLDB adapter did not exit after forced cleanup"
		}
	}
	if err != nil {
		return "could not confirm inferior cleanup: " + err.Error()
	}
	return ""
}
