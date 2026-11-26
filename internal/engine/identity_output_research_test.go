//go:build darwin || linux || windows

package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Routilet/c-observer/internal/observer"
)

// Record protocol field coverage without preserving values, debugger paths or environment.
var identityFields map[string]int

func auditIdentityDAP(data json.RawMessage) json.RawMessage {
	var body struct{ Variables []map[string]json.RawMessage }
	if err := json.Unmarshal(data, &body); err != nil || body.Variables == nil {
		return data
	}
	strip := os.Getenv("COBS_STRIP_DECLARATIONS") == "1"
	for _, v := range body.Variables {
		identityFields["total"]++
		for _, field := range []string{"evaluateName", "namedVariables", "declarationLocationReference", "valueLocationReference"} {
			if _, exists := v[field]; exists {
				identityFields[field]++
			}
		}
		var extensions map[string]json.RawMessage
		if json.Unmarshal(v["$__lldb_extensions"], &extensions) == nil && len(extensions["declaration"]) != 0 {
			identityFields["extension.declaration"]++
		}
		if strip {
			delete(v, "$__lldb_extensions")
		}
	}
	if strip {
		encoded, err := json.Marshal(body)
		if err == nil {
			return encoded
		}
	}
	return data
}

func TestIdentityOutputAudit(t *testing.T) {
	path := os.Getenv("COBS_IDENTITY_AUDIT")
	if path == "" {
		t.Skip("run research/identity-output/run.py")
	}
	config := realConfig(t)
	config.KeepLocations, config.ArrayRanges = nil, nil
	config.Limits.MaxOutputBytes = 128
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	enc := json.NewEncoder(out)
	for _, sample := range []struct {
		fixture, label, input string
		args                  []string
	}{
		{"birthday_observe.c", "birthday", "2\nAda 28\nLin 1\n1\n2 2\n", nil},
		{"output_buffering.c", "default", "\n\n", nil},
		{"output_buffering.c", "line", "\n\n", []string{"-DBUFFER_MODE=_IOLBF"}},
		{"output_buffering.c", "none", "\n\n", []string{"-DBUFFER_MODE=_IONBF"}},
		{"output_buffering.c", "default-abort", "x", nil},
		{"output_buffering.c", "line-abort", "x", []string{"-DBUFFER_MODE=_IOLBF"}},
		{"output_buffering.c", "none-abort", "x", []string{"-DBUFFER_MODE=_IONBF"}},
		{"output_buffering.c", "truncated", "\ny", []string{"-DBUFFER_MODE=_IONBF"}},
	} {
		original, err := os.ReadFile("../../testdata/programs/" + sample.fixture)
		if err != nil {
			t.Fatal(err)
		}
		text := string(original)
		config.SourceMap, config.VisibleSourcePaths = nil, nil
		if sample.label == "birthday" {
			visible := filepath.Join(config.WorkDir, "birthday.c")
			if err := os.WriteFile(visible, original, 0600); err != nil {
				t.Fatal(err)
			}
			config.SourceMap = map[string]string{"birthday-observe.c": visible}
			config.VisibleSourcePaths = []string{visible}
			preset := "#include <stdio.h>\nstatic int read_int(void) { int n = 0; scanf(\"%d\", &n); return n; }\n"
			if path := os.Getenv("COBS_IDENTITY_PRESET"); path != "" {
				expanded, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				preset = string(expanded) + "\n"
			}
			text = "#line 1 \"preset/io.h\"\n" + preset + "#line 1 \"birthday-observe.c\"\n" + text
		}
		if err := os.WriteFile(config.SourcePath, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		config.Stdin, config.CompilerArgs = []byte(sample.input), sample.args
		identityFields = make(map[string]int)
		start := time.Now()
		result := realEngine().Run(context.Background(), config)
		if err := enc.Encode(map[string]any{"case": sample.label, "fields": identityFields, "ms": time.Since(start).Milliseconds(), "run": result}); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %s (%d stops), fields=%v, output=%q", sample.label, result.Reason, len(result.Snapshots), identityFields, result.Output.Stdout)
		if sample.label == "birthday" {
			verifyBirthdayAudit(t, result)
		}
		want := observer.Exited
		if sample.input == "x" {
			want = observer.Signaled
		}
		if result.Reason != want || result.CleanupError != "" {
			t.Errorf("%s: unexpected result: %s %s (%s) %v", sample.label, result.Reason, result.Message, result.CleanupError, result.Diagnostics)
		}
	}
}

func verifyBirthdayAudit(t *testing.T, result observer.RunResult) {
	t.Helper()
	var previous observer.Variable
	found := false
	for _, snapshot := range result.Snapshots {
		var day observer.Variable
		for _, frame := range snapshot.Frames {
			if frame.Location.Function != "main" {
				continue
			}
			for _, v := range frame.Variables {
				if v.Name == "i" && v.IdentityReliable {
					t.Errorf("loop-local i marked reliable: %s", v.ID)
				}
				if v.Name != "birthdays" {
					continue
				}
				for _, element := range v.Children {
					if element.Index == nil || *element.Index != 2 {
						continue
					}
					for _, field := range element.Children {
						if field.Name != "date" {
							continue
						}
						for _, member := range field.Children {
							if member.Name == "day" {
								day = member
							}
						}
					}
				}
			}
		}
		if previous.Value == "1" && day.Value == "3" && previous.IdentityReliable &&
			day.IdentityReliable && day.ID != "" && day.ID == previous.ID {
			found = true
		}
		previous = day
	}
	if !found {
		t.Error("birthday replay has no reliable 1 → 3 day transition")
	}
}
