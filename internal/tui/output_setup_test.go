package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestExactOutputComparison(t *testing.T) {
	rows := CompareOutput([]byte("1 2\n\n3\n"), []byte("1 2\r\n\n3"))
	if len(rows) != 3 || rows[0].Equal || !rows[1].Equal || rows[2].Equal {
		t.Fatalf("bad comparison: %+v", rows)
	}
	if !strings.Contains(rows[0].Expected, "·") || !strings.Contains(rows[0].Actual, "\\r") || !strings.Contains(rows[0].Actual, "↵") {
		t.Fatal("invisible characters omitted")
	}
	rows = CompareOutput([]byte("a\n"), []byte{})
	if len(rows) != 1 || rows[0].HasActual || rows[0].Equal {
		t.Fatal("missing line")
	}
	if visibleLine([]byte{0xff}) != "\\xff" {
		t.Fatal("invalid byte hidden")
	}
}
func TestImportAndCompletion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "中文.c"), []byte("int main(void){}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample.txt"), []byte("1\r\n2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := SetupState{path: newField("中")}
	s.completePath(dir)
	if s.path.text != "中文.c" {
		t.Fatalf("completion: %s", s.path.text)
	}
	data, err := readSample(filepath.Join(dir, "sample.txt"))
	if err != nil || data != "1\r\n2\n" {
		t.Fatal("sample must preserve bytes")
	}
	m := NewSetupPreview(fixture(t, "wrong-index"))
	m.config.WorkDir = dir
	m.setup.focus = 1
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	if !m.setup.importing {
		t.Fatal("import form not opened")
	}
	m.setup.importPath = newField("sample.txt")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.setup.importing || m.setup.input.text != data {
		t.Fatal("import failed")
	}
}

func TestCompanionSampleComparison(t *testing.T) {
	for _, c := range []struct {
		expected, actual, mode string
		want bool
	}{
		{"1\n", "1\r\n", "exact", true},
		{"1\n", "1 \t\r\n\n", "trim-line-end", true},
		{"1 2", "1\u00a0\uFEFF2\r\n", "tokens", true},
		{"1\n", "2\r\n", "exact", false},
	} {
		if got, applied := sampleEqual([]byte(c.expected), []byte(c.actual), c.mode); !applied || got != c.want {
			t.Fatalf("%+v: %v %v", c, got, applied)
		}
	}
	if _, applied := sampleEqual([]byte{0xff}, []byte{0xff}, "tokens"); applied {
		t.Fatal("invalid UTF-8 must remain a byte comparison")
	}
}
