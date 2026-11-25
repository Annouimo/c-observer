//go:build darwin || linux

package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Annouimo/c-observer/internal/observer"
)

func realConfig(t *testing.T) observer.LaunchConfig {
	t.Helper()
	if os.Getenv("C_OBSERVER_REAL_DEBUGGER") != "1" && os.Getenv("C_OBSERVER_REAL_GDB") != "1" {
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

func realEngine() observer.Engine {
	if os.Getenv("C_OBSERVER_REAL_GDB") == "1" {
		return GDBEngine{}
	}
	return Engine{}
}

func TestRealObservation(t *testing.T) {
	config := realConfig(t)
	config.KeepLocations = nil
	result := realEngine().Run(context.Background(), config)
	if result.Reason != observer.Exited || result.ExitCode == nil || *result.ExitCode != 0 ||
		result.CleanupError != "" {
		t.Fatalf("run ended with %s: %s (cleanup: %s, diagnostics: %+v, snapshots: %d)",
			result.Reason, result.Message, result.CleanupError, result.Diagnostics, len(result.Snapshots))
	}
	if string(result.Output.Stdout) != "Ada:29:1\n" || string(result.Output.Stderr) != "done\n" {
		t.Fatalf("unexpected output: %+v", result.Output)
	}
	callback, before, after := false, false, false
	var beforeID, afterID string
	resumedMain := false
	for i, snapshot := range result.Snapshots {
		if snapshot.ID != i || snapshot.OutputOffsets.StdoutBytes > len(result.Output.Stdout) ||
			snapshot.OutputOffsets.StderrBytes > len(result.Output.Stderr) {
			t.Fatalf("invalid snapshot %d", i)
		}
		resumedMain = resumedMain || callback && snapshot.Location.Function == "main"
		callback = callback || snapshot.Location.Function == "compare"
		if snapshot.Location.File == "" {
			t.Fatalf("unexpected stop in library code: %+v", snapshot.Location)
		}
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
						if frame.IdentityReliable && v.IdentityReliable && field.IdentityReliable {
							switch field.Value {
							case "28":
								beforeID = field.ID
							case "29":
								afterID = field.ID
							}
						}
					}
				}
			}
		}
	}
	if !callback || !resumedMain || !before || !after {
		t.Fatalf("missing real captures: callback=%v resumedMain=%v day28=%v day29=%v snapshots=%d diagnostics=%+v",
			callback, resumedMain, before, after, len(result.Snapshots), result.Diagnostics)
	}
	if beforeID == "" || beforeID != afterID {
		t.Fatalf("birthday day has no stable identity: 28=%q 29=%q", beforeID, afterID)
	}
	if path := os.Getenv("C_OBSERVER_RECORD"); path != "" {
		record := struct {
			Synthetic bool               `json:"synthetic"`
			Platform  string             `json:"platform"`
			Stdin     string             `json:"stdin"`
			Run       observer.RunResult `json:"run"`
		}{false, runtime.GOOS + "/" + runtime.GOARCH, string(config.Stdin), result}
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRealFailureAndStop(t *testing.T) {
	config := realConfig(t)
	t.Run("crash", func(t *testing.T) {
		config.Stdin = []byte("crash 0\n")
		result := realEngine().Run(context.Background(), config)
		if result.Reason != observer.Signaled || len(result.Snapshots) == 0 || result.CleanupError != "" {
			t.Fatalf("unexpected crash result: %s %s, cleanup %s, %d snapshots",
				result.Reason, result.Message, result.CleanupError, len(result.Snapshots))
		}
	})
	t.Run("capture limit", func(t *testing.T) {
		config.Stdin = []byte("spin 0\n")
		config.Limits.MaxSnapshots = 1
		result := realEngine().Run(context.Background(), config)
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
		go func() { done <- realEngine().Run(ctx, config) }()
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
		result := realEngine().Run(context.Background(), config)
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
		if result := realEngine().Run(context.Background(), config); result.Reason != observer.InvalidConfig {
			t.Fatalf("unexpected reason: %s", result.Reason)
		}
	}
}

func TestRealIdentityBoundaries(t *testing.T) {
	config := realConfig(t)
	source, err := os.ReadFile("../../testdata/programs/identity.c")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.SourcePath, source, 0600); err != nil {
		t.Fatal(err)
	}
	config.KeepLocations = nil
	config.ArrayRanges = nil
	result := realEngine().Run(context.Background(), config)
	if result.Reason != observer.Exited {
		t.Fatalf("run failed: %s %s", result.Reason, result.Message)
	}
	iterations := map[string]bool{}
	var recurseDepth int
	for _, snapshot := range result.Snapshots {
		depth := 0
		for _, frame := range snapshot.Frames {
			if frame.Location.Function == "recurse" {
				depth++
				if frame.IdentityReliable {
					t.Fatal("recursive frame was marked reliable")
				}
			}
			for _, variable := range frame.Variables {
				if strings.HasPrefix(variable.Name, "shadow") && variable.IdentityReliable &&
					(variable.Declaration == nil || variable.Declaration.Line != 10) {
					t.Fatalf("nested shadow was marked reliable: %s", variable.ID)
				}
				if variable.Name == "iteration" {
					if variable.IdentityReliable {
						t.Fatal("loop-local object was marked reliable")
					}
					iterations[variable.Value] = true
				}
			}
		}
		recurseDepth = max(recurseDepth, depth)
	}
	if !iterations["0"] || !iterations["1"] || recurseDepth != 3 {
		t.Fatalf("missing real loop or recursion stops: iterations=%v depth=%d", iterations, recurseDepth)
	}
}

func TestRealGlobalsAndStatics(t *testing.T) {
	config := realConfig(t)
	data, err := os.ReadFile("../../testdata/programs/globals.c")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.SourcePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	config.KeepLocations = nil
	config.ArrayRanges = []observer.ArrayRange{{Path: "grid", Start: 900, Count: 1}}
	result := realEngine().Run(context.Background(), config)
	if result.Reason != observer.Exited || !result.Session.Capabilities.Globals {
		t.Fatalf("run: %s %s, capabilities %+v", result.Reason, result.Message, result.Session.Capabilities)
	}
	visitID, totalID, cellID := "", "", ""
	visits := map[string]bool{}
	sawTotal, sawCell, sawRecursion := false, false, false
	for _, snapshot := range result.Snapshots {
		seen := map[string]bool{}
		for _, v := range snapshot.Globals {
			if !v.IdentityReliable || seen[v.ID] {
				t.Fatalf("duplicate or unreliable global at stop %d: %+v", snapshot.ID, v)
			}
			seen[v.ID] = true
			switch v.Name {
			case "visits":
				if v.Scope != observer.FunctionStaticScope || visitID != "" && visitID != v.ID {
					t.Fatalf("function static has invalid scope or identity: %+v", v)
				}
				visitID = v.ID
				visits[v.Value] = true
				frames := 0
				for _, f := range snapshot.Frames {
					if f.Location.Function == "visit" {
						frames++
					}
				}
				sawRecursion = sawRecursion || frames == 3
			case "total":
				if v.Scope != observer.FileStaticScope || totalID != "" && totalID != v.ID {
					t.Fatalf("file static has invalid scope or identity: %+v", v)
				}
				totalID = v.ID
				sawTotal = sawTotal || v.Value == "6"
			case "grid":
				if v.Scope != observer.GlobalScope || len(v.Children) != 1 ||
					v.Children[0].Index == nil || *v.Children[0].Index != 900 {
					t.Fatalf("global grid has invalid scope or page: %+v", v)
				}
				for _, cell := range v.Children[0].Children {
					if cell.Index != nil && *cell.Index == 1 {
						if !cell.IdentityReliable || cellID != "" && cellID != cell.ID {
							t.Fatalf("global cell has invalid identity: %+v", cell)
						}
						cellID = cell.ID
						sawCell = sawCell || cell.Value == "6"
					}
				}
			}
		}
	}
	if !visits["1"] || !visits["3"] || !sawTotal || !sawCell || !sawRecursion {
		t.Fatalf("missing actual observations: visits=%v total=%v cell=%v recursion=%v",
			visits, sawTotal, sawCell, sawRecursion)
	}
}

func TestRealMappedLineAndNonzeroExit(t *testing.T) {
	config := realConfig(t)
	original := filepath.Join(config.WorkDir, "student.c")
	logical := "#include <stdio.h>\nint main(void) {\n    puts(\"mapped\");\n    return 7;\n}\n"
	if err := os.WriteFile(original, []byte(logical), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.SourcePath, []byte("#line 1 \"logical.c\"\n"+logical), 0600); err != nil {
		t.Fatal(err)
	}
	config.VisibleSourcePaths = []string{original}
	config.SourceMap = map[string]string{"logical.c": original, filepath.Join(config.WorkDir, "logical.c"): original}
	config.KeepLocations, config.ArrayRanges = nil, nil
	result := realEngine().Run(context.Background(), config)
	if result.Reason != observer.NonzeroExit || result.ExitCode == nil || *result.ExitCode != 7 ||
		string(result.Output.Stdout) != "mapped\n" || result.CleanupError != "" {
		t.Fatalf("unexpected result: %s %s, exit=%v, stdout=%q, cleanup=%q",
			result.Reason, result.Message, result.ExitCode, result.Output.Stdout, result.CleanupError)
	}
	for _, s := range result.Snapshots {
		if s.Location.File == original && s.Location.Line == 3 {
			return
		}
	}
	t.Fatalf("did not observe the mapped source line: %+v", result.Snapshots)
}
