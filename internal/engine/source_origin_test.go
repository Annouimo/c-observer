package engine

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMappedOriginsRequireFrozenSource(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "frozen.c")
	compiled := "#line 1 \"preset/helper.c\"\nint named_helper;\n" +
		"#line 1 \"student.c\"\nint correct;\nint changed;\n"
	origins := buildSourceOrigins(compiled, dir,
		map[string]string{user: "int correct;\nint original;\n"},
		map[string]string{"student.c": user, "preset/helper.c": user})
	for _, test := range []struct {
		token, presumed string
		presumedLine    int
		file            string
		line            int
	}{
		{"correct", "", 0, user, 1},
		{"correct", "student.c", 1, user, 1},
		{"correct", "student.c", 2, "", 0},
		{"correct", "other.c", 1, "", 0},
		{"changed", "", 0, "", 0},
		{"named_helper", "", 0, "", 0},
	} {
		file, line := origins.resolve(strings.Index(compiled, test.token), test.presumed, test.presumedLine)
		if file != test.file || line != test.line {
			t.Errorf("%s (%s:%d): got %q:%d, want %q:%d", test.token, test.presumed, test.presumedLine,
				file, line, test.file, test.line)
		}
	}
}
