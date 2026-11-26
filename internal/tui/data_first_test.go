package tui

import (
	"strings"
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
	"github.com/charmbracelet/lipgloss"
)

func TestReplayStepUsesChronologicalOrigin(t *testing.T) {
	run := testRun(testArray("a", []int{1}, 0), testArray("a", []int{2}, 0), testArray("a", []int{3}, 0))
	run.Snapshots[0].Location.Line = 12
	run.Snapshots[1].Location.Line = 3
	run.Snapshots[2].Location.Line = 8
	r := NewReplay(run)
	if r.Step().HasInterval() {
		t.Fatal("initial state is not an execution interval")
	}
	r.Seek(2)
	step := r.Step()
	if !step.HasInterval() || step.Origin.Line != 3 || step.After.Location.Line != 8 {
		t.Fatalf("wrong chronological origin: %+v", step)
	}
	r.Seek(0)
	r.Seek(2)
	if r.Step() != step {
		t.Fatal("jump changed interval semantics")
	}
	r.Seek(1)
	if r.Step().Origin.Line != 12 {
		t.Fatal("backward navigation must not reverse or decrement source lines")
	}
	r.Seek(3)
	if r.Step().HasInterval() || r.Step().After != nil {
		t.Fatal("end must not invent a post-state")
	}
}

func TestDataFirstRootCoverage(t *testing.T) {
	run := testRun(testArray("matrixA", []int{1, 2, 3}, 0))
	frame := &run.Snapshots[0].Frames[0]
	frame.Variables = append(frame.Variables,
		testArray("matrixB", []int{4, 5, 6}, 0),
		testArray("matrixAB", []int{7, 8, 9}, 0),
		observer.Variable{ID: "i", Name: "i", Kind: observer.Scalar, Status: observer.Observed, Value: "12", IdentityReliable: true})
	m := NewPreview(Fixture{Synthetic: true, Run: run})
	if m.focus != 1 {
		t.Fatal("default must be data-first")
	}
	for _, size := range [][2]int{{80, 24}, {120, 30}, {160, 40}} {
		m.width, m.height = size[0], size[1]
		view := m.View()
		for _, name := range []string{"matrixA", "matrixB", "matrixAB", "i = 12"} {
			if !strings.Contains(view, name) {
				t.Fatalf("missing %s at %v", name, size)
			}
		}
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > m.width {
				t.Fatalf("width overflow: %q", line)
			}
		}
		if len(strings.Split(view, "\n")) != m.height {
			t.Fatal("height mismatch")
		}
	}
}
