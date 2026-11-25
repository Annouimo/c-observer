package observer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntheticFixtures(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/snapshots/*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("fixtures: %v, %v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var fixture struct {
				Synthetic      bool      `json:"synthetic"`
				Run            RunResult `json:"run"`
				ExpectedStdout *string   `json:"expected_stdout"`
			}
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatal(err)
			}
			if !fixture.Synthetic || fixture.Run.Session.Backend != "synthetic" {
				t.Fatal("invalid synthetic run")
			}
			validateRun(t, fixture.Run)
			switch filepath.Base(path) {
			case "wrong-index.json":
				if fixture.Run.Reason != Exited || string(fixture.Run.Output.Stdout) != "17 25 30 40\n" || fixture.ExpectedStdout == nil || *fixture.ExpectedStdout == string(fixture.Run.Output.Stdout) {
					t.Fatal("incorrect wrong-index scenario")
				}
			case "compile-failed.json":
				if fixture.Run.Reason != CompileFailed || len(fixture.Run.Snapshots) != 0 ||
					len(fixture.Run.Diagnostics) == 0 || fixture.Run.ExitCode != nil {
					t.Fatal("compile failure should return diagnostics with no snapshots")
				}
			case "cancelled.json":
				if fixture.Run.Reason != Cancelled || len(fixture.Run.Snapshots) != 1 || fixture.Run.ExitCode != nil {
					t.Fatal("cancellation should preserve the completed snapshot")
				}
				frames := fixture.Run.Snapshots[0].Frames
				if len(frames) != 1 || len(frames[0].Variables) != 1 {
					t.Fatal("cancelled snapshot should retain the observed array")
				}
				page := frames[0].Variables[0]
				if page.ChildrenComplete || len(page.Children) != 1 ||
					page.Children[0].Index == nil || *page.Children[0].Index != 9000 {
					t.Fatal("selected page should preserve its actual index and partial status")
				}
			case "limited.json":
				if fixture.Run.Reason != LimitReached || fixture.Run.ExitCode != nil {
					t.Fatal("truncated capture must not imply exit")
				}
			}
		})
	}
}

func validateRun(t *testing.T, run RunResult) {
	t.Helper()
	if run.Reason == "" {
		t.Fatal("missing end reason")
	}
	sources := make(map[string]int)
	for _, source := range run.Session.Sources {
		sources[source.Path] = len(strings.Split(source.Text, "\n"))
	}
	previous := OutputOffsets{}
	for i, snapshot := range run.Snapshots {
		if snapshot.ID != i {
			t.Fatalf("snapshot ID %d, want %d", snapshot.ID, i)
		}
		if loc := snapshot.Location; loc.File != "" {
			if lines, exists := sources[loc.File]; !exists || loc.Line < 0 || loc.Line > lines {
				t.Fatalf("invalid mapped location: %+v", loc)
			}
		}
		offsets := snapshot.OutputOffsets
		if offsets.StdoutBytes < previous.StdoutBytes || offsets.StdoutBytes > len(run.Output.Stdout) ||
			offsets.StderrBytes < previous.StderrBytes || offsets.StderrBytes > len(run.Output.Stderr) {
			t.Fatalf("invalid output offsets: %+v after %+v", offsets, previous)
		}
		previous = offsets
		for _, frame := range snapshot.Frames {
			if frame.ID == "" {
				t.Fatal("missing frame ID")
			}
			validateVariables(t, frame.Variables)
		}
		validateVariables(t, snapshot.Globals)
	}
}

func validateVariables(t *testing.T, vars []Variable) {
	t.Helper()
	ids := make(map[string]bool)
	for _, v := range vars {
		if v.ID == "" || ids[v.ID] {
			t.Fatalf("missing or duplicate sibling identity: %q", v.ID)
		}
		ids[v.ID] = true
		if v.Status != Observed && v.Status != NotCaptured && v.Status != Unavailable {
			t.Fatalf("invalid value status: %q", v.Status)
		}
		if v.Kind == Array && v.ArrayLength != nil {
			if len(v.Children) > *v.ArrayLength || (v.ChildrenComplete && len(v.Children) != *v.ArrayLength) {
				t.Fatalf("incorrect array completeness: %s", v.Name)
			}
			indices := make(map[int]bool)
			for _, child := range v.Children {
				if child.Index == nil || *child.Index < 0 || *child.Index >= *v.ArrayLength || indices[*child.Index] {
					t.Fatalf("invalid or duplicate array index: %s", child.Name)
				}
				indices[*child.Index] = true
			}
		}
		validateVariables(t, v.Children)
	}
}
