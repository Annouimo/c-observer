//go:build darwin || linux || windows

package engine

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Routilet/c-observer/internal/observer"
)

// Used only by the overlay in research/input-progress/run.py.
type inputProbe struct {
	ID     int    `json:"id"`
	Line   int    `json:"line"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
	Micros int64  `json:"micros"`
}

var inputExperiment struct {
	mode   string
	probes []inputProbe
	pid    int
}

func inputExpr() string {
	stdin := "(void *)stdin"
	switch runtime.GOOS {
	case "darwin":
		stdin = "(void *)__stdinp"
	case "windows":
		stdin = "((void *(*)(unsigned int))__acrt_iob_func)(0)"
	}
	if inputExperiment.mode == "ftello" {
		// off_t is 64-bit on the Linux and Mac architectures under test.
		return "(long long)ftello(" + stdin + ")"
	}
	if inputExperiment.mode == "fixture-null" || inputExperiment.mode == "fixture-ready" {
		stdin = "(void *)probe_stdin"
	}
	if inputExperiment.mode == "timeout" {
		if runtime.GOOS == "windows" {
			return "((void (*)(unsigned long))Sleep)(20000)"
		}
		return "(unsigned int)sleep(20)"
	}
	return "(long)ftell(" + stdin + ")"
}

func shouldProbeInput(snapshot observer.Snapshot) bool {
	if snapshot.Location.Function != "main" || inputExperiment.mode == "off" {
		return false
	}
	return inputExperiment.mode != "fixture-ready" || snapshot.Location.Line >= 26
}

func researchDAP(c *capture, frames []stackFrame, snapshot observer.Snapshot, _ string, crash bool) error {
	if crash || len(frames) == 0 || !shouldProbeInput(snapshot) {
		return nil
	}
	var result struct {
		Value string `json:"result"`
	}
	start := time.Now()
	err := c.ask("evaluate", map[string]any{"expression": inputExpr(), "frameId": frames[0].ID, "context": "watch"}, &result)
	probe := inputProbe{ID: snapshot.ID, Line: snapshot.Location.Line, Result: result.Value, Micros: time.Since(start).Microseconds()}
	return recordInputProbe(probe, err)
}

func researchMI(m *gdbMI, request func(string) error, snapshot observer.Snapshot, path string, crash bool) error {
	if crash || !shouldProbeInput(snapshot) {
		return nil
	}
	code := "print('INPUT:' + str(gdb.parse_and_eval(" + strconv.Quote(inputExpr()) + ")))"
	if inputExperiment.mode == "readonly" {
		code = "exec(" + strconv.Quote(strings.Join([]string{
			"import os, json",
			"f = gdb.parse_and_eval('stdin').dereference()",
			"pid = gdb.selected_inferior().pid",
			"fd = int(f['_fileno'])",
			"data = open('/proc/%d/fdinfo/%d' % (pid, fd)).read()",
			"ospos = int(next(line.split()[1] for line in data.splitlines() if line.startswith('pos:')))",
			"unread = int(f['_IO_read_end']) - int(f['_IO_read_ptr'])",
			"backup = int(f['_IO_save_end']) - int(f['_IO_save_base']) if int(f['_flags']) & 0x100 else 0",
			"original = os.path.samefile('/proc/%d/fd/%d' % (pid, fd), " + strconv.Quote(path) + ")",
			"print('INPUT:' + json.dumps(dict(os=ospos, unread=unread, backup=backup, offset=ospos-unread-backup, original=original, eof=bool(int(f['_flags']) & 0x10), error=bool(int(f['_flags']) & 0x20))))",
		}, "\n")) + ")"
	}
	if inputExperiment.mode == "timeout" {
		code = "print('INFERIOR_PID:' + str(gdb.selected_inferior().pid)); gdb.flush(); " + code
	}
	start := time.Now()
	err := request(miConsole("python " + code))
	if _, tail, ok := strings.Cut(m.console, "INFERIOR_PID:"); ok {
		digits, _, _ := strings.Cut(tail, "\n")
		inputExperiment.pid, _ = strconv.Atoi(digits)
	}
	_, value, _ := strings.Cut(m.console, "INPUT:")
	value, _, _ = strings.Cut(value, "\n")
	probe := inputProbe{ID: snapshot.ID, Line: snapshot.Location.Line, Result: value, Micros: time.Since(start).Microseconds()}
	return recordInputProbe(probe, err)
}

func recordInputProbe(probe inputProbe, err error) error {
	if err != nil {
		probe.Error = err.Error()
	}
	inputExperiment.probes = append(inputExperiment.probes, probe)
	// No safe way to step after a debugger request timed out. Keep the
	// snapshot already collected and let the existing cleanup run.
	if inputExperiment.mode == "timeout" {
		return err
	}
	return nil
}

func TestInputProgressResearch(t *testing.T) {
	output := os.Getenv("COBS_INPUT_OUTPUT")
	if output == "" {
		t.Skip("run research/input-progress/run.py to enable real debugger probes")
	}
	backend := os.Getenv("COBS_INPUT_BACKEND")
	repeat, _ := strconv.Atoi(os.Getenv("COBS_INPUT_REPEAT"))
	if repeat < 1 {
		t.Fatal("repeat must be >= 1")
	}
	var engine observer.Engine = Engine{}
	if backend == "gdb" {
		engine = GDBEngine{}
	}
	dir := t.TempDir()
	source, err := filepath.Abs("../../testdata/programs/input_progress.c")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(dir, "input_progress.c")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replacement.txt"), []byte("other\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	type inputCase struct {
		name, input string
		mode, limit int
	}
	cases := []inputCase{
		{"lf", "12 34\nabc\nX\n", '1', 0},
		{"crlf", "12 34\r\nabc\r\nX\r\n", '1', 0},
		{"mixed-utf8", "12\r\n34\n中\r\nX", '1', 0},
		{"empty", "", '1', 0},
		{"edge", " \t\n\n? A\nB word xyz123\n", 'F', 0},
		{"until-eof", "123456789abcdef\n\n中\nLAST", 'D', 0},
		{"many", strings.Repeat("1 ", 650), 'Y', 0},
		{"crash", "", 'C', 0},
		{"limit", strings.Repeat("1 ", 20), 'Y', 5},
		{"cancel", "", 'L', 0},
		{"timeout", "12 34\nabc\nX\n", '1', 0},
	}
	if runtime.GOOS == "linux" {
		cases = append(cases, inputCase{"pipe", "", 'P', 0})
	}
	courseSource := os.Getenv("COBS_INPUT_COURSE_SOURCE")
	if courseSource != "" {
		raw, err := os.ReadFile(os.Getenv("COBS_INPUT_COURSE_STDIN"))
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, inputCase{name: "course", input: string(raw)})
	}
	selected := os.Getenv("COBS_INPUT_CASE")
	found := false
	for _, sample := range cases {
		if selected != "" && selected != sample.name {
			continue
		}
		found = true
		var baseline observer.RunResult
		modes := []string{"off", "ftell"}
		if sample.name == "lf" {
			if runtime.GOOS != "windows" {
				modes = append(modes, "ftello")
			}
			modes = append(modes, "fixture-null", "fixture-ready")
		}
		if backend == "gdb" && runtime.GOOS == "linux" {
			modes = append(modes, "readonly")
		}
		if sample.name == "timeout" {
			modes = []string{"off", "timeout"}
		}
		tries := 1
		if sample.name == "many" {
			tries = repeat
		}
		for run := 0; run < tries; run++ {
			for _, mode := range modes {
				inputExperiment.mode, inputExperiment.probes, inputExperiment.pid = mode, nil, 0
				cfg := observer.LaunchConfig{SourcePath: source, WorkDir: dir, Stdin: []byte(sample.input), CompilerArgs: []string{fmt.Sprintf("-DMODE=%d", sample.mode)}}
				if sample.name == "course" {
					cfg.SourcePath, cfg.WorkDir, cfg.CompilerArgs = courseSource, filepath.Dir(courseSource), nil
				}
				cfg.Limits.MaxSnapshots = sample.limit
				if sample.name == "cancel" {
					cfg.Limits.MaxSnapshots = 100000
				}
				ctx, cancel := context.WithCancel(context.Background())
				start := time.Now()
				var timer *time.Timer
				if sample.name == "cancel" {
					timer = time.AfterFunc(500*time.Millisecond, cancel)
				}
				result := engine.Run(ctx, cfg)
				if timer != nil {
					timer.Stop()
				}
				cancel()
				ms := time.Since(start).Milliseconds()
				if mode == "off" {
					baseline = result
				}
				positions := func(r observer.RunResult) []int {
					lines := make([]int, len(r.Snapshots))
					for i, s := range r.Snapshots {
						lines[i] = s.Location.Line
					}
					return lines
				}
				record := map[string]any{"backend": backend, "case": sample.name, "input_hex": hex.EncodeToString(cfg.Stdin), "mode": mode, "run": run + 1,
					"ms": ms, "reason": result.Reason, "message": result.Message, "exit": result.ExitCode, "cleanup": result.CleanupError,
					"stdout": string(result.Output.Stdout), "stderr": string(result.Output.Stderr), "lines": positions(result), "probes": inputExperiment.probes,
					"same_lines": reflect.DeepEqual(positions(result), positions(baseline)), "same_result": result.Reason == baseline.Reason && reflect.DeepEqual(result.ExitCode, baseline.ExitCode) && reflect.DeepEqual(result.Output, baseline.Output),
					"diagnostics": result.Diagnostics}
				if pid := inputExperiment.pid; pid > 0 {
					record["inferior_pid"] = pid
					record["inferior_after_cleanup"] = "gone"
					if err := verifyInferiorExited(pid); err != nil {
						record["inferior_after_cleanup"] = err.Error()
					}
				}
				if err := enc.Encode(record); err != nil {
					t.Fatal(err)
				}
				t.Logf("%s %s %s: %s, %d snapshots, %d probes, %dms", backend, sample.name, mode, result.Reason, len(result.Snapshots), len(inputExperiment.probes), ms)
				if mode == "off" && (result.Reason == observer.CompileFailed || result.Reason == observer.BackendFailed || result.Reason == observer.InvalidConfig) {
					t.Errorf("baseline failed: %s (%s %v)", sample.name, result.Message, result.Diagnostics)
				}
				if sample.name == "lf" && mode == "off" &&
					(result.Reason != observer.Exited || !strings.Contains(string(result.Output.Stdout), "values:12,34,abc")) {
					t.Errorf("LF fixture baseline unexpected: %s, %q", result.Reason, result.Output.Stdout)
				}
			}
		}
	}
	if !found {
		t.Fatalf("unknown case %q", selected)
	}
}
