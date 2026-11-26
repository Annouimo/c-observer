package tui

import (
	"strings"
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
)

func TestStudentDisplayCorrespondence(t *testing.T) {
	variable := func(value string) observer.Variable {
		return observer.Variable{Name: "i", ID: "temporary", Type: "int", Kind: observer.Scalar, Status: observer.Observed, Value: value}
	}
	for _, tc := range []struct {
		name string
		edit func(*observer.RunResult)
		want bool
	}{
		{"untrusted", func(r *observer.RunResult) {}, true},
		{"temporary ID changes", func(r *observer.RunResult) {
			r.Snapshots[1].Frames[0].Variables[0].ID = "other"
		}, true},
		{"untrusted frames", func(r *observer.RunResult) {
			r.Snapshots[0].Frames[0].IdentityReliable = false
			r.Snapshots[1].Frames[0].IdentityReliable = false
		}, true},
		{"different declarations", func(r *observer.RunResult) {
			for i := range r.Snapshots {
				r.Snapshots[i].Frames[0].Variables[0].Declaration = &observer.SourceLocation{File: "main.c", Line: i + 1}
			}
		}, false},
		{"different known objects", func(r *observer.RunResult) {
			for i := range r.Snapshots {
				r.Snapshots[i].Frames[0].Variables[0].IdentityReliable = true
			}
			r.Snapshots[1].Frames[0].Variables[0].ID = "different"
		}, false},
		{"different known frames", func(r *observer.RunResult) {
			r.Snapshots[1].Frames[0].ID = "different"
		}, false},
		{"different function", func(r *observer.RunResult) {
			r.Snapshots[0].Frames[0].IdentityReliable = false
			r.Snapshots[1].Frames[0].IdentityReliable = false
			r.Snapshots[1].Frames[0].Location.Function = "other"
		}, false},
		{"duplicate name", func(r *observer.RunResult) {
			r.Snapshots[0].Frames[0].Variables = append(r.Snapshots[0].Frames[0].Variables, variable("9"))
		}, false},
		{"different type", func(r *observer.RunResult) {
			r.Snapshots[1].Frames[0].Variables[0].Type = "long"
		}, false},
		{"not captured", func(r *observer.RunResult) {
			r.Snapshots[1].Frames[0].Variables[0].Status = observer.NotCaptured
		}, false},
		{"truncated", func(r *observer.RunResult) {
			r.Snapshots[0].Frames[0].Variables[0].ValueTruncated = true
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := testRun(variable("1"), variable("2"))
			tc.edit(&run)
			r := NewReplay(run)
			r.Seek(1)
			row := r.Rows(nil)[0]
			if row.Changed != tc.want || row.ReliableChange {
				t.Fatalf("display correspondence: %+v", row)
			}
			if (len(r.Index.Changes) != 0) != tc.want {
				t.Fatal("visible filter differs from display")
			}
			if row.Changed && (row.Before != "1" || row.Value != "2") {
				t.Fatal("incorrect values")
			}
			for _, entries := range r.Index.Objects {
				for _, entry := range entries {
					if entry.Changed {
						t.Fatal("candidate entered reliable history")
					}
				}
			}
		})
	}
}

func TestStudentFinalOutput(t *testing.T) {
	run := testRun(testArray("a", []int{0}, 0), testArray("a", []int{1}, 0))
	run.Output.Stdout = []byte("3\n")
	expected := "3\n"
	m := NewPreview(Fixture{Synthetic: true, ExpectedStdout: &expected, Run: run})
	if got := m.View(); !strings.Contains(got, "最终输出") || !strings.Contains(got, "样例输出一致") || strings.Contains(got, "尚无已捕获输出") {
		t.Fatalf("main view: %s", got)
	}
	m = key(m, "i")
	if m.consoleTab != 4 || !strings.Contains(m.View(), "最终输出对照") || !strings.Contains(m.View(), "样例输出一致") {
		t.Fatal("I must open final comparison")
	}
	if got := m.consoleTextFor(5); got != "（空）" {
		t.Fatalf("step output must remain empty: %q", got)
	}
	m.replay.Run.Output.StdoutTruncated = true
	if m.sampleStatus() != "输出不完整" {
		t.Fatal("truncated output must not be judged")
	}
	m.replay.Run.Reason = observer.TimedOut
	if m.sampleStatus() != "运行未正常结束" {
		t.Fatal("partial execution must not be judged")
	}
}
