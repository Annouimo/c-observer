package tui

import (
	"strings"
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
	tea "github.com/charmbracelet/bubbletea"
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

func TestDataFirstStress(t *testing.T) {
	f := fixture(t, "data-first-stress")
	for _, size := range [][2]int{{80, 24}, {120, 30}, {160, 40}} {
		for p := 0; p < len(f.Run.Snapshots); p += 2 {
			m := NewPreview(f)
			m.width, m.height = size[0], size[1]
			m.seek(p)
			before := m.View()
			m.seek(p + 1)
			after := m.View()
			for _, root := range rootRows(m.replay.Rows(nil)) {
				if !strings.Contains(before, root.Name) || !strings.Contains(after, root.Name) {
					t.Errorf("%v step %d missing root %s", size, p+2, root.Name)
				}
			}
			for _, view := range []string{before, after} {
				if len(strings.Split(view, "\n")) != m.height {
					t.Fatalf("%v step %d height mismatch", size, p+2)
				}
				for _, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > m.width {
						t.Fatalf("%v step %d width overflow: %q", size, p+2, line)
					}
				}
			}
			// Returning by a different route does not change the rendered diff.
			m.seek(0)
			m.seek(p + 1)
			if m.View() != after {
				t.Fatalf("%v step %d rendering depends on navigation", size, p+2)
			}
		}
	}
}

func TestDataFirstLongValueAndNestedDetails(t *testing.T) {
	m := NewPreview(fixture(t, "data-first-stress"))
	m.width, m.height = 80, 24
	m.seek(3)
	if !strings.Contains(m.View(), "123456789012345679") {
		t.Fatal("wide numeric array values should be legible in the preview")
	}
	m.varCursor = 3
	m = key(m, "d")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.mode != "value" || !strings.Contains(m.View(), "123456789012345679") {
		t.Fatal("cannot inspect full captured integer")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.mode != "detail" {
		t.Fatal("Esc should return from the full value to details")
	}
	m.mode = ""
	m.seek(5)
	m.varCursor = 1
	if view := m.View(); !strings.Contains(view, "values") || !strings.Contains(view, "indices") {
		t.Fatal("structure should preview both nested arrays")
	}
	m = key(m, "d")
	m = key(m, "t")
	row, ok := m.inspectedRow()
	if !ok {
		t.Fatal("structure unavailable")
	}
	found := false
	for _, line := range m.detailTable(row).Rows {
		if line[1].Path == "record.values[2]" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("nested captured value not reachable")
	}
}
