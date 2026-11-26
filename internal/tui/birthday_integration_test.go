//go:build darwin || linux || windows

package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
	"github.com/charmbracelet/lipgloss"
)

// This representative course fixture is run through the real debugger and
// a #line-mapped assembled source. It is not the maintainer's private record.
func TestMappedBirthdayReplay(t *testing.T) {
	if os.Getenv("C_OBSERVER_REAL_DEBUGGER") != "1" && os.Getenv("C_OBSERVER_REAL_GDB") != "1" {
		t.Skip("real debugger test is opt-in")
	}
	backend, compiler, debugger, label := realFrontendEngine()
	dir := t.TempDir()
	original := filepath.Join(dir, "student.c")
	source := filepath.Join(dir, "observe.c")
	program := `struct Birthday {
    char name[24];
    struct Date { int day; } date;
};
int main(void) {
    int n = read_int();
    struct Birthday birthdays[10010] = {0};
    for (int i = 1; i <= n; i++) {
        scanf("%23s", birthdays[i].name);
        birthdays[i].date.day = read_int();
    }
    int q = read_int();
    for (int i = 0; i < q; i++) {
        int index = read_int(), x = read_int();
        birthdays[index].date.day += x;
    }
    for (int i = 1; i <= n; i++)
        printf("%s %d\n", birthdays[i].name, birthdays[i].date.day);
    return 0;
}
`
	preset := "#line 1 \"preset/io.h\"\n#include <stdio.h>\nstatic int read_int(void) { int n = 0; scanf(\"%d\", &n); return n; }\n"
	for path, text := range map[string]string{original: program, source: preset + "#line 1 \"student.c\"\n" + program} {
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	expected := []byte("Ada 28\nLin 3\n")
	cfg := observer.LaunchConfig{
		SourcePath: source, WorkDir: dir,
		VisibleSourcePaths: []string{original},
		SourceMap:          map[string]string{"student.c": original, filepath.Join(dir, "student.c"): original},
		Stdin:              []byte("2\nAda 28\nLin 1\n1\n2 2\n"),
		ExpectedStdout:     &expected, Compiler: compiler, Debugger: debugger,
	}
	result := backend.Run(context.Background(), cfg)
	actual := strings.ReplaceAll(string(result.Output.Stdout), "\r\n", "\n")
	if result.Reason != observer.Exited || result.CleanupError != "" || actual != string(expected) {
		t.Fatalf("birthday run: %s %s, output=%q, cleanup=%q, diagnostics=%+v", result.Reason, result.Message, actual, result.CleanupError, result.Diagnostics)
	}
	path, err := SaveRecord(t.TempDir(), captureRecord("0.1.1", cfg, result))
	if err != nil {
		t.Fatal(err)
	}
	record, err := LoadRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewRecorded(record)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range result.Snapshots {
		m.seek(i)
		for _, row := range m.replay.Rows(nil) {
			if row.Path != "birthdays[2].date.day" || !row.Changed || row.Before != "1" || row.Value != "3" {
				continue
			}
			if !row.Reliable || !row.Comparable {
				t.Fatal("birthday change has no reliable identity")
			}
			found = true
			if summary := m.comparisonSummary(100); !strings.Contains(summary, "birthdays[2].date.day: 1 → 3") {
				t.Fatalf("birthday change summary: %q", summary)
			}
			for _, size := range [][2]int{{80, 24}, {120, 30}, {160, 40}} {
				m.width, m.height = size[0], size[1]
				view := m.View()
				for _, want := range []string{"数据", "输入", "输出", "birthdays", "Ada", "Lin"} {
					if !strings.Contains(view, want) {
						t.Fatalf("%v missing %q:\n%s", size, want, view)
					}
				}
				for _, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > size[0] {
						t.Fatalf("%v width overflow: %q", size, line)
					}
				}
				if size[0] == 120 {
					t.Logf("real %s birthday replay at snapshot %d:\n%s", label, i+1, view)
				}
			}
		}
	}
	if !found {
		t.Fatal("no reliable birthday day transition 1 → 3")
	}
}
