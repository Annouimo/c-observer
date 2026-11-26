package tui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
	tea "github.com/charmbracelet/bubbletea"
)

func testRecord() ObservationRecord {
	expected := []byte{}
	run := observer.RunResult{
		Reason: observer.Exited,
		Session: observer.Session{Backend: "gdb-mi", Sources: []observer.SourceFile{{Path: "missing/main.c", Text: "int main(void) { return 0; }\n"}}},
		Snapshots: []observer.Snapshot{
			{ID: 0, Location: observer.SourceLocation{File: "missing/main.c", Line: 1}, OutputOffsets: observer.OutputOffsets{StdoutBytes: 1}},
			{ID: 1, Location: observer.SourceLocation{File: "missing/main.c", Line: 1}, OutputOffsets: observer.OutputOffsets{StdoutBytes: 3}},
		},
		Output: observer.OutputState{Stdout: []byte{0xff, 0, '\n'}, Stderr: []byte{0x1b}},
	}
	return captureRecord("0.1.1", observer.LaunchConfig{Stdin: []byte{0xff, '\r', '\n'}, ExpectedStdout: &expected, Comparison: "exact"}, run)
}

func TestObservationRecordRoundTrip(t *testing.T) {
	record := testRecord()
	dir := t.TempDir()
	path, err := SaveRecord(dir, record)
	if err != nil {
		t.Fatal(err)
	}
	other, err := SaveRecord(dir, record)
	if err != nil || path == other || filepath.Dir(path) != dir {
		t.Fatalf("unique export: %q %q %v", path, other, err)
	}
	got, err := LoadRecord(path)
	if err != nil || !reflect.DeepEqual(got, record) {
		t.Fatalf("round trip: %v, equal=%v", err, reflect.DeepEqual(got, record))
	}
	m, err := NewRecorded(got)
	if err != nil || m.Init() != nil || m.engine != nil || m.demo || !m.offline {
		t.Fatalf("offline model: %v", err)
	}
	if !bytes.Equal(m.config.Stdin, record.Stdin) || m.config.ExpectedStdout == nil || len(*m.config.ExpectedStdout) != 0 {
		t.Fatal("sample lost")
	}
	for i, size := range []int{1, 3, 3} {
		m.seek(i)
		out, _ := m.replay.Output()
		if len(out) != size || !strings.Contains(m.View(), "离线记录") {
			t.Fatalf("replay %d: %q", i, out)
		}
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil || next.(Model).replay.Position != 0 {
		t.Fatal("offline reset attempted to execute")
	}
}

func TestObservationRecordRejectsInvalid(t *testing.T) {
	for _, mutate := range []func(*ObservationRecord){
		func(r *ObservationRecord) { r.SchemaVersion++ },
		func(r *ObservationRecord) { r.Format = "" },
		func(r *ObservationRecord) { r.Run.Session.Backend = "synthetic" },
		func(r *ObservationRecord) { r.Run.Snapshots[0].OutputOffsets.StdoutBytes = -1 },
		func(r *ObservationRecord) { r.Run.Snapshots[1].OutputOffsets.StdoutBytes = 0 },
		func(r *ObservationRecord) { r.Run.Snapshots[1].OutputOffsets.StdoutBytes = 100 },
	} {
		r := testRecord()
		mutate(&r)
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = DecodeRecord(bytes.NewReader(b)); err == nil {
			t.Fatal("accepted invalid record")
		}
	}
	r := testRecord()
	r.Run.Snapshots = nil
	r.Run.Reason = observer.Cancelled
	r.ExpectedStdout = nil
	b, _ := json.Marshal(r)
	if _, err := DecodeRecord(bytes.NewReader(append(b, []byte(" {}")...))); err == nil {
		t.Fatal("accepted trailing JSON")
	}
	got, err := DecodeRecord(bytes.NewReader(b))
	if err != nil || got.ExpectedStdout != nil {
		t.Fatalf("valid partial record: %v", err)
	}
	m, err := NewRecorded(got)
	if err != nil || !m.replay.End() {
		t.Fatalf("empty history: %v", err)
	}
}

func TestObservationRecordExportConfirmation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	r := testRecord()
	m := NewLive(nil, observer.LaunchConfig{Stdin: r.Stdin, ExpectedStdout: r.ExpectedStdout, Comparison: r.Comparison}).WithVersion("0.1.1")
	next, _ := m.Update(runDone{r.Run})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = next.(Model)
	if m.mode != "record-confirm" {
		t.Fatal("confirmation missing")
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatal("w saved without confirmation")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.mode != "" {
		t.Fatal("cancel did not return")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.mode != "record-result" || m.exportPath == "" || m.notice != "" {
		t.Fatalf("export: %s %q", m.mode, m.notice)
	}
	got, err := LoadRecord(m.exportPath)
	if err != nil || got.ObserverVersion != "0.1.1" || !reflect.DeepEqual(got.Run, r.Run) {
		t.Fatalf("saved live result: %v", err)
	}
}
