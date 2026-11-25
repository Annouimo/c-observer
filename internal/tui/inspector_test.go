package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Annouimo/c-observer/internal/observer"
)

func TestArrayRecordTableAndHistory(t *testing.T) {
	f := fixture(t, "wrong-index")
	r := Replay{Run: f.Run, Position: 2}
	visible := r.Rows(map[string]bool{})
	if len(visible) == 0 || visible[0].Name != "a" || !visible[0].Collapsed {
		t.Fatal("array should start collapsed")
	}
	row := visible[0]
	table := TableFor(row, r.Rows(nil))
	if len(table.Headers) != 2 || table.Headers[1] != "score" || len(table.Rows) != 3 {
		t.Fatalf("table: %+v", table)
	}
	if !table.Rows[0][1].Changed || table.Rows[0][1].Text != "10 → 17" {
		t.Fatalf("change: %+v", table.Rows[0])
	}
	if table.Rows[1][1].Changed {
		t.Fatal("unchanged cell highlighted")
	}
	history := r.History(row)
	found := false
	for _, e := range history {
		if e.Position == 2 && e.Changed && strings.Contains(e.Text, "a[0].score") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing array history: %+v", history)
	}
	row.Reliable = false
	if len(r.History(row)) != 0 {
		t.Fatal("unreliable history")
	}
}
func TestArrayPageAndRange(t *testing.T) {
	f := fixture(t, "cancelled")
	r := Replay{Run: f.Run}
	row := r.Rows(nil)[0]
	table := TableFor(row, r.Rows(nil))
	if len(table.Rows) != 1 || table.Rows[0][0].Text != "[9000]" {
		t.Fatalf("page: %+v", table)
	}
	page, err := rangeFor(row, nil, "9000", "16", 16)
	if err != nil || page.Start != 9000 || page.Count != 16 || page.Path != "records" {
		t.Fatalf("range: %+v %v", page, err)
	}
	for _, pair := range [][2]string{{"-1", "16"}, {"0", "0"}, {"0", "17"}, {"999999999", "1"}, {"x", "1"}} {
		if _, err := rangeFor(row, nil, pair[0], pair[1], 16); err == nil {
			t.Fatalf("invalid range accepted: %v", pair)
		}
	}
}
func key(m Model, k string) Model {
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	return next.(Model)
}
func TestInspectorNavigation(t *testing.T) {
	m := NewPreview(fixture(t, "wrong-index"))
	m.focus = 1
	m.replay.Seek(2)
	m = key(m, "d")
	if m.mode != "detail" || !strings.Contains(m.View(), "10 → 17") {
		t.Fatal("missing detail")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.replay.Position != 3 || m.mode != "detail" {
		t.Fatal("detail must follow timeline")
	}
	m = key(m, "h")
	if m.mode != "history" {
		t.Fatal("history")
	}
	for i, e := range m.history {
		if e.Position == 2 {
			m.modalScroll = i
			break
		}
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.replay.Position != 2 || m.mode != "" {
		t.Fatal("history jump")
	}
	m.varCursor = 0
	m = key(m, "a")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.mode != "range" || !strings.Contains(m.notice, "合成预览") || len(m.config.ArrayRanges) != 0 {
		t.Fatal("preview must not pretend to collect")
	}
}

type fakeEngine struct{}

func (fakeEngine) Run(context.Context, observer.LaunchConfig) observer.RunResult {
	return observer.RunResult{Reason: observer.Cancelled}
}
func TestLiveRangeStartsNewRun(t *testing.T) {
	m := NewLive(fakeEngine{}, observer.LaunchConfig{})
	m.replay = Replay{Run: fixture(t, "cancelled").Run}
	m.focus = 1
	m = key(m, "a")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil || !m.running || m.mode != "" || len(m.config.ArrayRanges) != 1 || m.config.ArrayRanges[0].Start != 9000 {
		t.Fatal("range must launch new run")
	}
	if m.cancel != nil {
		m.cancel()
	}
}
func TestInspectorFitsTerminal(t *testing.T) {
	m := NewPreview(fixture(t, "wrong-index"))
	m.focus = 1
	m.replay.Seek(2)
	for _, mode := range []string{"d", "h", "a"} {
		for _, size := range [][2]int{{100, 30}, {86, 14}, {45, 14}} {
			x := m
			x.width, x.height = size[0], size[1]
			x = key(x, mode)
			lines := strings.Split(x.View(), "\n")
			if len(lines) > x.height {
				t.Fatalf("%s height %d > %d", mode, len(lines), x.height)
			}
			for _, l := range lines {
				if lipgloss.Width(l) > x.width {
					t.Fatalf("%s width %d > %d: %q", mode, lipgloss.Width(l), x.width, l)
				}
			}
		}
	}
}
