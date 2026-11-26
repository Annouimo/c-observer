package tui

import (
	"strings"
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestStringWithChildrenChange(t *testing.T) {
	makeString := func(value, char string, reliable bool) observer.Variable {
		return observer.Variable{ID: "name", Name: "name", Type: "char[25]", Kind: observer.String,
			Status: observer.Observed, Value: value, IdentityReliable: reliable,
			Children: []observer.Variable{{ID: "name/0", Name: "[0]", Type: "char", Kind: observer.Scalar,
				Status: observer.Observed, Value: char, IdentityReliable: reliable}}}
	}
	run := testRun(makeString("\"\"", "'\\0'", true), makeString("\"ling\"", "'l'", true))
	m := NewPreview(Fixture{Synthetic: true, Run: run})
	m.seek(1)
	rows := m.replay.Rows(nil)
	if !rows[0].Changed || !rows[0].Comparable || rows[0].Before != "\"\"" || !rows[1].Changed {
		t.Fatalf("missing logical string change: %+v", rows)
	}
	if label := m.comparisonSummary(100); !strings.Contains(label, "name: \"\" → \"ling\"") || strings.Contains(label, "name[0]") {
		t.Fatalf("string should be summarized once: %s", label)
	}
	for _, mutate := range []func(*observer.Variable){
		func(v *observer.Variable) { v.ValueTruncated = true },
	} {
		r := testRun(makeString("\"\"", "'\\0'", true), makeString("\"ling\"", "'l'", true))
		mutate(&r.Snapshots[1].Frames[0].Variables[0])
		p := NewPreview(Fixture{Synthetic: true, Run: r})
		p.seek(1)
		if p.replay.Rows(nil)[0].Changed {
			t.Fatal("reported truncated string change")
		}
	}
}

func TestComparisonCoverage(t *testing.T) {
	makeScalar := func(reliable bool) observer.Variable {
		return observer.Variable{ID: "n", Name: "n", Type: "int", Kind: observer.Scalar, Status: observer.Observed,
			Value: "2", IdentityReliable: reliable, ChildrenComplete: true}
	}
	for _, reliable := range []bool{false, true} {
		run := testRun(makeScalar(reliable), makeScalar(reliable))
		m := NewPreview(Fixture{Synthetic: true, Run: run})
		m.seek(1)
		want := "无可见变化"
		if got := m.comparisonSummary(80); !strings.Contains(got, want) {
			t.Fatalf("reliable=%v: %s", reliable, got)
		}
	}
}

func TestFoldOnlyKnownConsecutivePreviewFields(t *testing.T) {
	row := func(i string, known, changed bool) []Cell {
		return []Cell{{Text: i}, {Text: "0", PreviewValue: "0", PreviewKnown: known, Changed: changed}}
	}
	table := DataTable{Headers: []string{"下标", "day"}, Rows: [][]Cell{
		row("[0]", true, false), row("[1]", true, false), row("[2]", true, false),
		row("[3]", true, true), row("[4]", false, false), row("[5]", true, false),
		row("[7]", true, false), row("[8]", true, false), row("[9]", true, false),
	}}
	got, folded := foldPreviewRows(table)
	if !folded || len(got.Rows) != 5 || got.Rows[0][0].Text != "[0..2]≈" ||
		got.Rows[1][0].Text != "[3]" || got.Rows[2][0].Text != "[4]" ||
		got.Rows[3][0].Text != "[5]" || got.Rows[4][0].Text != "[7..9]≈" || len(table.Rows) != 9 {
		t.Fatalf("unsafe preview fold: %+v", got)
	}
}

func TestCompactWorkspaceStreamsAndMenu(t *testing.T) {
	run := testRun(testArray("a", []int{1, 2}, 0), testArray("a", []int{1, 3}, 0))
	run.Output.Stdout = []byte("  first\n  second")
	run.Snapshots[0].OutputOffsets.StdoutBytes = len("  first\n")
	run.Snapshots[1].OutputOffsets.StdoutBytes = len(run.Output.Stdout)
	m := NewPreview(Fixture{Synthetic: true, Stdin: "  2\n1 2\n", Run: run})
	m.seek(0)
	if got := strings.Join(m.outputPreview(30, 3), "\n"); !strings.Contains(got, "最终输出") || !strings.Contains(got, "  second") {
		t.Fatalf("final stdout: %q", got)
	}
	if got := m.consoleTextFor(5); !strings.Contains(got, "first") || strings.Contains(got, "second") {
		t.Fatalf("captured prefix: %q", got)
	}
	m.seek(1)
	for _, size := range [][2]int{{45, 14}, {60, 20}, {80, 24}, {120, 30}, {160, 40}} {
		m.width, m.height = size[0], size[1]
		view := m.View()
		for _, want := range []string{"输入", "输出", "数据", "a", "  2", "  first"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%v missing %q:\n%s", size, want, view)
			}
		}
		if len(strings.Split(view, "\n")) != size[1] {
			t.Fatalf("%v height mismatch", size)
		}
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > size[0] {
				t.Fatalf("%v width overflow: %q", size, line)
			}
		}
	}
	m = key(m, "?")
	if m.mode != "help" {
		t.Fatal("action menu missing")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := next.(Model); got.mode != "" || got.focus != 2 {
		t.Fatal("menu did not open input/output")
	}
}

func TestChangedPreviewRowsTakePriority(t *testing.T) {
	lines := []previewLine{{text: "title"}, {text: "columns"}, {text: "early", cells: 1},
		{text: "middle", cells: 1}, {text: "changed", cells: 1, priority: true}}
	got := visiblePreview(lines, 3)
	if len(got) != 3 || got[1].text != "columns" || got[2].text != "changed" {
		t.Fatalf("did not preserve context and changed row: %+v", got)
	}
}
