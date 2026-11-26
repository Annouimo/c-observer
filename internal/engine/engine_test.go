//go:build darwin || linux

package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Routilet/c-observer/internal/observer"
)

func realConfig(t *testing.T) observer.LaunchConfig {
	t.Helper()
	if os.Getenv("C_OBSERVER_REAL_DEBUGGER") != "1" {
		t.Skip("set C_OBSERVER_REAL_DEBUGGER=1 to run LLDB against real C programs")
	}
	dir := filepath.Join(t.TempDir(), "课程 sample")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/programs/observe.c")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "student program.c")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	line := 1
	for _, sourceLine := range strings.Split(string(data), "\n") {
		if strings.Contains(sourceLine, "int a = *(const int *)left") {
			break
		}
		line++
	}
	return observer.LaunchConfig{SourcePath: source, WorkDir: dir, Stdin: []byte("run 28\n"),
		KeepLocations: []observer.SourceLocation{{File: source, Line: line}},
		ArrayRanges:   []observer.ArrayRange{{Path: "birthdays", Start: 9000, Count: 1}},
	}
}

func TestRealObservation(t *testing.T) {
	config := realConfig(t)
	result := (Engine{}).Run(context.Background(), config)
	if result.Reason != observer.Exited || result.ExitCode == nil || *result.ExitCode != 0 ||
		result.CleanupError != "" {
		t.Fatalf("run ended with %s: %s (cleanup: %s, diagnostics: %+v, snapshots: %d)",
			result.Reason, result.Message, result.CleanupError, result.Diagnostics, len(result.Snapshots))
	}
	if string(result.Output.Stdout) != "Ada:29:1\n" || string(result.Output.Stderr) != "done\n" {
		t.Fatalf("unexpected output: %+v", result.Output)
	}
	callback, before, after := false, false, false
	for i, snapshot := range result.Snapshots {
		if snapshot.ID != i || snapshot.OutputOffsets.StdoutBytes > len(result.Output.Stdout) ||
			snapshot.OutputOffsets.StderrBytes > len(result.Output.Stderr) {
			t.Fatalf("invalid snapshot %d", i)
		}
		callback = callback || snapshot.Location.Function == "compare"
		for _, frame := range snapshot.Frames {
			for _, v := range frame.Variables {
				if v.Name != "birthdays" || len(v.Children) != 1 || v.Children[0].Index == nil ||
					*v.Children[0].Index != 9000 || v.ChildrenComplete {
					continue
				}
				for _, field := range v.Children[0].Children {
					if field.Name == "day" && field.Status == observer.Observed {
						before = before || field.Value == "28"
						after = after || field.Value == "29"
					}
				}
			}
		}
	}
	if !callback || !before || !after {
		t.Fatalf("missing real captures: callback=%v day28=%v day29=%v snapshots=%d diagnostics=%+v",
			callback, before, after, len(result.Snapshots), result.Diagnostics)
	}
}

func TestRealFailureAndStop(t *testing.T) {
	config := realConfig(t)
	t.Run("crash", func(t *testing.T) {
		config.Stdin = []byte("crash 0\n")
		result := (Engine{}).Run(context.Background(), config)
		if result.Reason != observer.Signaled || len(result.Snapshots) == 0 || result.CleanupError != "" {
			t.Fatalf("unexpected crash result: %s %s, cleanup %s, %d snapshots",
				result.Reason, result.Message, result.CleanupError, len(result.Snapshots))
		}
	})
	t.Run("capture limit", func(t *testing.T) {
		config.Stdin = []byte("spin 0\n")
		config.Limits.MaxSnapshots = 1
		result := (Engine{}).Run(context.Background(), config)
		if result.Reason != observer.LimitReached || len(result.Snapshots) != 1 || result.CleanupError != "" {
			t.Fatalf("unexpected limit result: %s %s cleanup %s, %d snapshots",
				result.Reason, result.Message, result.CleanupError, len(result.Snapshots))
		}
		config.Limits.MaxSnapshots = 0
	})
	t.Run("cancel", func(t *testing.T) {
		config.Stdin = []byte("spin 0\n")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan observer.RunResult, 1)
		go func() { done <- (Engine{}).Run(ctx, config) }()
		pidPath := filepath.Join(config.WorkDir, "inferior.pid")
		deadline := time.NewTimer(8 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		pid := 0
		for pid == 0 {
			select {
			case result := <-done:
				t.Fatalf("run finished before entering infinite loop: %s %s", result.Reason, result.Message)
			case <-deadline.C:
				t.Fatal("inferior did not enter infinite loop")
			case <-ticker.C:
				data, err := os.ReadFile(pidPath)
				if err == nil {
					pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
				}
			}
		}
		cancel()
		select {
		case result := <-done:
			if result.Reason != observer.Cancelled || result.CleanupError != "" || len(result.Snapshots) == 0 {
				t.Fatalf("unexpected cancellation result: %s %s cleanup %s, %d snapshots",
					result.Reason, result.Message, result.CleanupError, len(result.Snapshots))
			}
			if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
				t.Fatalf("inferior %d after cancellation: %v", pid, err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("cancellation did not complete within the cleanup budget")
		}
	})
	t.Run("compile error", func(t *testing.T) {
		if err := os.WriteFile(config.SourcePath, []byte("int main( {\n"), 0600); err != nil {
			t.Fatal(err)
		}
		result := (Engine{}).Run(context.Background(), config)
		if result.Reason != observer.CompileFailed || len(result.Diagnostics) == 0 || len(result.Snapshots) != 0 {
			t.Fatalf("unexpected compile error: %+v", result)
		}
	})
}

func TestInvalidConfig(t *testing.T) {
	for _, config := range []observer.LaunchConfig{
		{},
		{Limits: observer.Limits{MaxSnapshots: -1}},
		{ArrayRanges: []observer.ArrayRange{{Path: "call()", Count: 1}}},
	} {
		if result := (Engine{}).Run(context.Background(), config); result.Reason != observer.InvalidConfig {
			t.Fatalf("unexpected reason: %s", result.Reason)
		}
	}
}
