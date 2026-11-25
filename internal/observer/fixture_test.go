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
				Synthetic bool `json:"synthetic"`
				Events []Event `json:"events"`
				ExpectedStdout *string `json:"expected_stdout"`
			}
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatal(err)
			}
			if !fixture.Synthetic || len(fixture.Events) < 2 {
				t.Fatal("missing synthetic marker or events")
			}
			first := fixture.Events[0]
			if first.Kind != StartedEvent || first.Session == nil || first.Session.Backend != "synthetic" {
				t.Fatal("invalid start")
			}
			sources := make(map[string]int)
			for _, source := range first.Session.Sources {
				sources[source.Path] = len(strings.Split(source.Text, "\n"))
			}
			next := 0
			for _, e := range fixture.Events[1:len(fixture.Events)-1] {
				if e.Kind != SnapshotEvent || e.Snapshot == nil {
					t.Fatal("unexpected event in fixture body")
				}
				s := e.Snapshot
				if s.ID != next || s.Location.Line < 1 || s.Location.Line > sources[s.Location.File] {
					t.Fatalf("invalid snapshot: %+v", s)
				}
				next++
				for _, frame := range s.Frames {
					if frame.ID == "" {
						t.Fatal("missing frame ID")
					}
					validateVariables(t, frame.Variables)
				}
			}
			last := fixture.Events[len(fixture.Events)-1]
			if last.Kind != FinishedEvent || last.Result == nil {
				t.Fatal("missing final result")
			}
			switch filepath.Base(path) {
			case "wrong-index.json":
				if last.Result.Reason != Exited || string(last.Result.Output.Stdout) != "17 25 30 40\n" || fixture.ExpectedStdout == nil || *fixture.ExpectedStdout == string(last.Result.Output.Stdout) {
					t.Fatal("incorrect wrong-index scenario")
				}
			case "limited.json":
				if last.Result.Reason != LimitReached || last.Result.ExitCode != nil {
					t.Fatal("truncated capture must not imply exit")
				}
			}
		})
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
			for _, child := range v.Children {
				if child.Index == nil || *child.Index < 0 || *child.Index >= *v.ArrayLength {
					t.Fatalf("invalid array index: %s", child.Name)
				}
			}
		}
		validateVariables(t, v.Children)
	}
}
