package tui

import (
	"os"
	"testing"

	"github.com/Annouimo/c-observer/internal/observer"
)

func fixture(t *testing.T, name string) Fixture {
	t.Helper()
	f, err := os.Open("../../testdata/snapshots/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	v, err := DecodeFixture(f)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestReplayEndAndOutput(t *testing.T) {
	f := fixture(t, "wrong-index")
	r := Replay{Run: f.Run}
	r.Seek(999)
	if !r.End() || r.Current() != nil || r.Previous() != nil {
		t.Fatal("end is not a snapshot")
	}
	out, _ := r.Output()
	if string(out) != "17 25 30 40\n" {
		t.Fatalf("%q", out)
	}
	r.Seek(-1)
	if r.Position != 0 {
		t.Fatal("seek must clamp")
	}
}
func TestChanges(t *testing.T) {
	f := fixture(t, "wrong-index")
	r := Replay{Run: f.Run, Position: 2}
	found := false
	for _, row := range r.Rows(nil) {
		if row.Name == "score" && row.Before == "10" && row.Value == "17" && row.Changed {
			found = true
		}
	}
	if !found {
		t.Fatal("missing changed struct field")
	}
	r.Run.Snapshots[2].Frames[0].IdentityReliable = false
	for _, row := range r.Rows(nil) {
		if row.Changed {
			t.Fatal("unreliable identity reported change")
		}
	}
}
func TestEmptyAndCancelled(t *testing.T) {
	f := fixture(t, "compile-failed")
	r := Replay{Run: f.Run}
	if !r.End() || len(r.Rows(nil)) != 0 {
		t.Fatal("empty history")
	}
	f = fixture(t, "cancelled")
	r = Replay{Run: f.Run}
	for _, row := range r.Rows(nil) {
		if row.Name == "[9000]" {
			return
		}
	}
	t.Fatal("array index lost")
}
func TestBadOffset(t *testing.T) {
	r := observer.RunResult{Reason: observer.Exited, Snapshots: []observer.Snapshot{{OutputOffsets: observer.OutputOffsets{StdoutBytes: -1}}}}
	if ValidateRun(r) == nil {
		t.Fatal("invalid offset accepted")
	}
}
func TestSafeText(t *testing.T) {
	if got := SafeText("a\x1b[31m\nb\u202e", false); got != "a\\u001b[31m\\nb\\u202e" {
		t.Fatalf("%q", got)
	}
}
