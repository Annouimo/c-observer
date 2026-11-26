package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func testPtr(n int) *int { return &n }
func testArray(id string, vals []int, start int) observer.Variable {
	a := observer.Variable{ID: id, Name: id, Type: "int[100010]", Kind: observer.Array, Status: observer.Observed, IdentityReliable: true, ArrayLength: testPtr(100010)}
	for i, n := range vals {
		j := i + start
		a.Children = append(a.Children, observer.Variable{ID: fmt.Sprintf("%d", j), Name: fmt.Sprintf("[%d]", j), Index: testPtr(j), Type: "int", Kind: observer.Scalar, Status: observer.Observed, IdentityReliable: true, Value: fmt.Sprint(n), ChildrenComplete: true})
	}
	return a
}
func testRun(vars ...observer.Variable) observer.RunResult {
	run := observer.RunResult{Reason: observer.Exited}
	for i, v := range vars {
		loc := observer.SourceLocation{File: "main.c", Line: 3, Function: "main"}
		run.Snapshots = append(run.Snapshots, observer.Snapshot{ID: i, Location: loc, Frames: []observer.Frame{{ID: "main", IdentityReliable: true, Location: loc, Variables: []observer.Variable{v}}}})
	}
	return run
}
func TestAdaptiveFocus(t *testing.T) {
	zeros := make([]int, 16)
	data := append([]int(nil), zeros...)
	data[1] = 3
	data[5] = 7
	r := NewReplay(testRun(testArray("a", zeros, 0), testArray("a", data, 0)))
	row := r.Rows(nil)[0]
	f := r.FocusFor(row)
	if !f.Automatic || f.Rows.Lo != 0 || f.Rows.Hi != 7 {
		t.Fatalf("focus: %+v", f)
	}
	r.Seek(1)
	if f != r.FocusFor(r.Rows(nil)[0]) {
		t.Fatal("focus changes across replay")
	}
	tab := focusTable(TableFor(r.Rows(nil)[0], r.Rows(nil)), f)
	if len(tab.Rows) != 8 || tab.Rows[3][1].Text != "0" {
		t.Fatalf("zero lost: %+v", tab)
	}
	m := Model{replay: r}
	if len(m.dataTable(r.Rows(nil)[0], r.Rows(nil)).Rows) != 8 {
		t.Fatal("focus not applied")
	}
	m.toggleFull(r.Rows(nil)[0])
	if len(m.dataTable(r.Rows(nil)[0], r.Rows(nil)).Rows) != 16 {
		t.Fatal("full range lost")
	}
}
func TestFocusFallbackAndNonPrefix(t *testing.T) {
	v := make([]int, 16)
	w := append([]int(nil), v...)
	w[9] = 2
	r := NewReplay(testRun(testArray("a", v, 9000), testArray("a", w, 9000)))
	f := r.FocusFor(r.Rows(nil)[0])
	if !f.Automatic || f.Rows.Lo != 9007 || f.Rows.Hi != 9011 {
		t.Fatalf("non-prefix: %+v", f)
	}
	r = NewReplay(testRun(testArray("a", v, 0), testArray("a", v, 0)))
	if r.FocusFor(r.Rows(nil)[0]).Automatic {
		t.Fatal("no change must not crop")
	}
	b := testArray("a", w, 0)
	b.Children[9].IdentityReliable = false
	r = NewReplay(testRun(testArray("a", v, 0), b))
	if r.FocusFor(r.Rows(nil)[0]).Automatic {
		t.Fatal("unreliable identity must not crop")
	}
}
func TestFocusMissingAndSparse(t *testing.T) {
	t1 := DataTable{Headers: []string{"下标", "值"}, Rows: [][]Cell{{{Text: "[0]"}, {Text: "3"}}, {{Text: "[100000]"}, {Text: "7"}}}}
	f := FocusRange{Rows: Bounds{Lo: 0, Hi: 100000, Valid: true}}
	got := focusTable(t1, f)
	if len(got.Rows) != 3 || got.Rows[1][1].Text != "未记录" {
		t.Fatalf("sparse gap: %+v", got)
	}
	f.Rows.Hi = 2
	got = focusTable(t1, f)
	if len(got.Rows) != 3 || got.Rows[2][1].Text != "未记录" {
		t.Fatal("missing value lost")
	}
}
func TestMatrixFocus(t *testing.T) {
	matrix := func(changed bool) observer.Variable {
		m := observer.Variable{ID: "m", Name: "m", Type: "int[1000][1000]", Kind: observer.Array, Status: observer.Observed, IdentityReliable: true}
		for i := 0; i < 8; i++ {
			data := make([]int, 8)
			if changed && i == 4 {
				data[3] = 1
			}
			row := testArray(fmt.Sprintf("row%d", i), data, 0)
			row.Index = testPtr(i)
			row.Name = fmt.Sprintf("[%d]", i)
			m.Children = append(m.Children, row)
		}
		return m
	}
	r := NewReplay(testRun(matrix(false), matrix(true)))
	f := r.FocusFor(r.Rows(nil)[0])
	if !f.Automatic || f.Rows.Lo != 3 || f.Rows.Hi != 5 || f.Cols.Lo != 2 || f.Cols.Hi != 4 {
		t.Fatalf("matrix focus %+v", f)
	}
	r.Seek(1)
	table := focusTable(TableFor(r.Rows(nil)[0], r.Rows(nil)), f)
	if len(table.Rows) != 3 || len(table.Headers) != 4 || !table.Rows[1][2].Changed {
		t.Fatalf("matrix table: %+v", table)
	}
}
func TestFilteredNavigationAndStack(t *testing.T) {
	m := NewPreview(fixture(t, "wrong-index"))
	m.setFilter(1)
	if len(m.navigation) == 0 {
		t.Fatal("missing changes")
	}
	first := m.navigation[0]
	m.navigate(1)
	if m.replay.Position != first {
		t.Fatal("filtered navigation")
	}
	m.setFilter(0)
	m = key(m, "c")
	if m.focus != 0 {
		t.Fatal("source view")
	}
	m = key(m, "j")
	m = key(m, "3")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.replay.Position != 2 || m.mode != "" {
		t.Fatal("jump")
	}
	run := testRun(testArray("a", []int{1}, 0), testArray("a", []int{2}, 0))
	for i := range run.Snapshots {
		run.Snapshots[i].Frames = append(run.Snapshots[i].Frames, observer.Frame{ID: "caller", IdentityReliable: true, Location: observer.SourceLocation{File: "caller.c", Line: 5}, Variables: []observer.Variable{testArray("b", []int{i}, 0)}})
	}
	m.replay = NewReplay(run)
	m = key(m, "s")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.replay.Frame != 1 || m.replay.Rows(nil)[0].Name != "b" {
		t.Fatal("frame selection")
	}
	m.seek(1)
	if m.replay.Frame != 1 || !m.replay.Rows(nil)[1].Changed {
		t.Fatal("caller change or identity lost")
	}
}
func TestWorkspaceDimensions(t *testing.T) {
	m := NewPreview(fixture(t, "wrong-index"))
	m.replay.Seek(2)
	for _, size := range [][2]int{{100, 30}, {86, 14}, {45, 14}} {
		for focus := 0; focus < 3; focus++ {
			x := m
			x.width, x.height = size[0], size[1]
			x.focus = focus
			lines := strings.Split(x.View(), "\n")
			if len(lines) > x.height {
				t.Fatalf("height %d > %d", len(lines), x.height)
			}
			for _, line := range lines {
				if lipgloss.Width(line) > x.width {
					t.Fatalf("width %d > %d: %q", lipgloss.Width(line), x.width, line)
				}
			}
		}
	}
}

func TestManualMatrixRange(t *testing.T) {
	matrix := observer.Variable{ID: "m", Name: "m", Kind: observer.Array, Type: "int[4][4]", Status: observer.Observed, IdentityReliable: true}
	for i := 0; i < 4; i++ {
		row := testArray(fmt.Sprint(i), []int{1, 2, 3, 4}, 0)
		row.Index = testPtr(i)
		row.Name = fmt.Sprintf("[%d]", i)
		matrix.Children = append(matrix.Children, row)
	}
	m := Model{replay: NewReplay(testRun(matrix))}
	row := m.replay.Rows(nil)[0]
	m.openDisplayRange(row)
	if m.displayFields != 4 {
		t.Fatal("matrix must offer both axes")
	}
	m.displayInput = [4]string{"1", "2", "2", "3"}
	m.handleNumeric("enter")
	if m.mode != "" {
		t.Fatal("valid manual range rejected")
	}
	table := m.dataTable(row, m.replay.Rows(nil))
	if len(table.Rows) != 2 || len(table.Headers) != 3 || table.Headers[1] != "[2]" || table.Rows[0][0].Text != "[1]" {
		t.Fatalf("range: %+v", table)
	}
	m.toggleFull(row)
	if len(m.dataTable(row, m.replay.Rows(nil)).Rows) != 4 {
		t.Fatal("full view failed")
	}
	m.toggleFull(row)
	if _, ok := m.manualRanges[row.Key]; ok {
		t.Fatal("auto range not restored")
	}
}
func TestActualPreviewText(t *testing.T) {
	m := NewPreview(fixture(t, "adaptive-array"))
	s, err := m.PreviewText(100, 26, 6)
	if err != nil || strings.Contains(s, "自动关注") || !strings.Contains(s, "合成数据预览") {
		t.Fatalf("preview: %v %s", err, s)
	}
	s, err = m.PreviewText(100, 26, 0)
	if err != nil || !strings.Contains(s, "输出一致") {
		t.Fatalf("ending: %v %s", err, s)
	}
}
