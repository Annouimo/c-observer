package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Annouimo/c-observer/internal/observer"
)

func TestTextFieldEditing(t *testing.T) {
	f := newField("abc")
	f.edit(tea.KeyMsg{Type: tea.KeyLeft}, true)
	f.edit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("中")}, true)
	if f.text != "ab中c" {
		t.Fatal(f.text)
	}
	f.edit(tea.KeyMsg{Type: tea.KeyEnter}, true)
	if f.text != "ab中\nc" {
		t.Fatal(f.text)
	}
	f.edit(tea.KeyMsg{Type: tea.KeyBackspace}, true)
	if f.text != "ab中c" {
		t.Fatal(f.text)
	}
}
func TestStandaloneConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "中文.c")
	if err := os.WriteFile(path, []byte("int main(void){return 0;}"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := observer.LaunchConfig{SourcePath: path, Stdin: []byte("1\n2\n")}
	m := NewSetup(fakeEngine{}, cfg)
	m.setup.withExpected = true
	m.setup.expected = newField("")
	if m.Init() != nil {
		t.Fatal("setup must not auto-run")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(Model)
	if cmd == nil || !m.running || m.settingUp {
		t.Fatal("setup must start run")
	}
	if m.config.SourcePath != path || m.config.WorkDir != dir || string(m.config.Stdin) != "1\n2\n" || m.config.ExpectedStdout == nil || len(*m.config.ExpectedStdout) != 0 {
		t.Fatalf("bad config: %+v", m.config)
	}
	if m.cancel != nil {
		m.cancel()
	}
}
func TestSetupPreview(t *testing.T) {
	m := NewSetupPreview(fixture(t, "wrong-index"))
	m.width, m.height = 45, 14
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(Model)
	if m.running || !m.settingUp || !strings.Contains(m.setup.message, "未连接") {
		t.Fatal("preview cannot execute")
	}
	for _, l := range strings.Split(m.View(), "\n") {
		if lipgloss.Width(l) > 45 {
			t.Fatalf("setup overflow: %q", l)
		}
	}
}
func TestSampledLineHistory(t *testing.T) {
	m := NewPreview(fixture(t, "wrong-index"))
	m.replay.Seek(1)
	m = key(m, "h")
	if m.mode != "line-history" || len(m.history) < 2 {
		t.Fatal("expected repeated pauses at the selected line")
	}
	m.modalScroll = len(m.history) - 1
	want := m.history[m.modalScroll].Position
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.replay.Position != want || m.focus != 0 || m.mode != "" {
		t.Fatal("line history jump")
	}
}
func TestMatrixUsesActualIndices(t *testing.T) {
	i, j := 2, 9
	p := VariableRow{Key: "m", Raw: observer.Variable{Kind: observer.Array}}
	rows := []VariableRow{
		p,
		{Key: "r", Parent: "m", Name: "[2]", Raw: observer.Variable{Kind: observer.Array, Index: &i}},
		{Key: "x", Parent: "r", Name: "[9]", Value: "7", Changed: true, Before: "4", Raw: observer.Variable{Kind: observer.Scalar, Index: &j}},
	}
	tab := TableFor(p, rows)
	if len(tab.Headers) != 2 || tab.Headers[1] != "[9]" || len(tab.Rows) != 1 || tab.Rows[0][0].Text != "[2]" || !tab.Rows[0][1].Changed {
		t.Fatalf("bad matrix table: %+v", tab)
	}
}
