//go:build darwin || linux || windows

package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
)

// TestLoopIdentityAudit writes one real, unmodified RunResult for offline
// analysis. It is opt-in so the normal suite never depends on a debugger.
func TestLoopIdentityAudit(t *testing.T) {
	out := os.Getenv("COBS_LOOP_AUDIT")
	if out == "" {
		t.Skip("set COBS_LOOP_AUDIT to write a real loop identity run")
	}
	fixture := os.Getenv("COBS_LOOP_FIXTURE")
	if fixture == "" {
		fixture = "loop_identity.c"
	}
	text, err := os.ReadFile("../../testdata/programs/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source := filepath.Join(root, fixture)
	if err := os.WriteFile(source, text, 0600); err != nil {
		t.Fatal(err)
	}
	result := realEngine().Run(context.Background(), observer.LaunchConfig{SourcePath: source, WorkDir: root})
	record := map[string]any{
		"backend": backendName(),
		"go":      runtime.Version(),
		"fixture": fixture,
		"run":     result,
		"workdir": root,
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s %s: %s, %d snapshots", fixture, backendName(), result.Reason, len(result.Snapshots))
}

func backendName() string {
	if os.Getenv("C_OBSERVER_REAL_GDB") == "1" {
		return "gdb"
	}
	return "lldb"
}
