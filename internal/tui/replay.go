package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/Routilet/c-observer/internal/observer"
 "github.com/Routilet/c-observer/internal/record"
)

// Fixture is a development-only record. It is never presented as a live run.
type Fixture struct {
	Synthetic      bool               `json:"synthetic"`
	Stdin          string             `json:"stdin"`
	ExpectedStdout *string            `json:"expected_stdout"`
	Run            observer.RunResult `json:"run"`
}

func DecodeFixture(r io.Reader) (Fixture, error) {
	var f Fixture
	if err := json.NewDecoder(r).Decode(&f); err != nil {
		return f, err
	}
	if !f.Synthetic || f.Run.Session.Backend != "synthetic" {
		return f, fmt.Errorf("preview requires an explicitly synthetic fixture")
	}
	if err := ValidateRun(f.Run); err != nil {
		return f, err
	}
	return f, nil
}

// ValidateRun is retained for the legacy TUI and fixture decoder.
func ValidateRun(run observer.RunResult) error { return record.ValidateRun(run) }

// Replay position n is a synthetic END screen, not a debugger snapshot.
type Replay struct {
	Run      observer.RunResult
	Position int
	Frame    int // selected stack frame; 0 is the executing frame
	Index    *HistoryIndex
}

func (r *Replay) Move(delta int) { r.Seek(r.Position + delta) }
func (r *Replay) Seek(pos int) {
	var frameID string
	if s := r.Current(); s != nil && r.Frame >= 0 && r.Frame < len(s.Frames) && s.Frames[r.Frame].IdentityReliable {
		frameID = s.Frames[r.Frame].ID
	}
	r.Position = max(0, min(pos, len(r.Run.Snapshots)))
	r.Frame = 0
	if frameID != "" {
		if s := r.Current(); s != nil {
			for i, f := range s.Frames {
				if f.IdentityReliable && f.ID == frameID {
					r.Frame = i
					break
				}
			}
		}
	}
}
func (r Replay) ViewedLocation() observer.SourceLocation {
	if s := r.Current(); s != nil {
		if r.Frame > 0 && r.Frame < len(s.Frames) {
			return s.Frames[r.Frame].Location
		}
		return s.Location
	}
	return observer.SourceLocation{}
}
func NewReplay(run observer.RunResult) Replay {
	return Replay{Run: run, Index: BuildHistoryIndex(run)}
}
func (r Replay) End() bool { return r.Position == len(r.Run.Snapshots) }
func (r Replay) Current() *observer.Snapshot {
	if r.Position < 0 || r.Position >= len(r.Run.Snapshots) {
		return nil
	}
	return &r.Run.Snapshots[r.Position]
}
func (r Replay) Previous() *observer.Snapshot {
	if r.Position <= 0 || r.End() {
		return nil
	}
	return &r.Run.Snapshots[r.Position-1]
}
func (r Replay) Output() (stdout, stderr []byte) {
	if s := r.Current(); s != nil {
		return r.Run.Output.Stdout[:s.OutputOffsets.StdoutBytes], r.Run.Output.Stderr[:s.OutputOffsets.StderrBytes]
	}
	return r.Run.Output.Stdout, r.Run.Output.Stderr
}

// SafeText escapes terminal control sequences and invalid bytes. Newlines can be
// preserved for source/console, never for single-line debugger values.
func SafeText(s string, multiline bool) string {
	var b strings.Builder
	for _, r := range strings.ToValidUTF8(s, "\ufffd") {
		switch {
		case r == '\n' && multiline:
			b.WriteRune(r)
		case r == '\t':
			b.WriteString("    ")
		case r == '\r':
			b.WriteString("\\r")
		case r == '\n':
			b.WriteString("\\n")
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r):
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

type VariableRow struct {
	Key, Name, Type, Value, Detail, Before string
	Depth                                  int
	Changed, Expandable, Collapsed         bool
	Reliable, Comparable, ReliableChange   bool
	Path                                   string
	Parent                                 string
	Raw                                    observer.Variable
}

// A display correspondence is not a claim of continuous C object lifetime.
// Prefer proven identity. Otherwise compare unique names/indices within a
// compatible parent and frame; known conflicting declarations/IDs win.
func correspondenceKey(v observer.Variable) string {
	if v.Index != nil {
		return fmt.Sprintf("index:%d", *v.Index)
	}
	return "name:" + v.Name
}

func compatibleObservation(a, b observer.Variable) bool {
	if a.Kind != b.Kind || a.Type != b.Type {
		return false
	}
	if a.Scope != "" && b.Scope != "" && a.Scope != b.Scope {
		return false
	}
	if a.IdentityReliable && b.IdentityReliable && a.ID != b.ID {
		return false
	}
	if a.Declaration != nil && b.Declaration != nil {
		x, y := a.Declaration, b.Declaration
		if x.File != "" && y.File != "" && x.File != y.File || x.Line > 0 && y.Line > 0 && x.Line != y.Line {
			return false
		}
	}
	return true
}

func compatibleFrames(current, previous []observer.Frame, depth int) bool {
	if len(current) != len(previous) || depth >= len(previous) {
		return false
	}
	if current[depth].Location.Function == "" || current[depth].Location.File == "" {
		return false
	}
	for i := depth; i < len(current); i++ {
		a, b := current[i], previous[i]
		if a.Location.Function != b.Location.Function || a.Location.File != b.Location.File ||
			a.IdentityReliable && b.IdentityReliable && a.ID != b.ID {
			return false
		}
		// A different caller position is evidence against correspondence.
		if i > depth && a.Location.Line != b.Location.Line {
			return false
		}
	}
	return true
}

// Rows compares adjacent observed states. ReliableChange is reserved for
// object history and automatic focusing; Changed also includes display matches.
func (r Replay) Rows(collapsed map[string]bool) []VariableRow {
	s := r.Current()
	if s == nil {
		return nil
	}
	p := r.Previous()
	var rows []VariableRow
	var walk func([]observer.Variable, []observer.Variable, string, string, int, bool, bool)
	walk = func(vars, old []observer.Variable, parent, path string, depth int, reliable, identity bool) {
		oldByID := make(map[string]observer.Variable)
		oldByName := make(map[string]observer.Variable)
		oldCounts, counts := map[string]int{}, map[string]int{}
		for _, v := range vars {
			counts[correspondenceKey(v)]++
		}
		for _, v := range old {
			k := correspondenceKey(v)
			oldByName[k] = v
			oldCounts[k]++
			if v.ID != "" && v.IdentityReliable {
				oldByID[v.ID] = v
			}
		}
		for i, v := range vars {
			key := fmt.Sprintf("%s/%d:%s", parent, i, v.ID)
			stable := identity && v.ID != "" && v.IdentityReliable
			if stable {
				key = fmt.Sprintf("%s/%q", parent, v.ID)
			}
			objectPath := v.Name
			if path != "" {
				if v.Index != nil {
					objectPath = fmt.Sprintf("%s[%d]", path, *v.Index)
				} else {
					objectPath = path + "." + v.Name
				}
			}
			o, ok := oldByID[v.ID]
			strict := reliable && v.ID != "" && v.IdentityReliable && ok && compatibleObservation(v, o)
			match := strict
			if !match {
				k := correspondenceKey(v)
				if counts[k] == 1 && oldCounts[k] == 1 && (v.Name != "" || v.Index != nil) {
					o = oldByName[k]
					match = compatibleObservation(v, o)
				}
			}
			row := VariableRow{Key: key, Name: SafeText(v.Name, false), Type: SafeText(v.Type, false), Depth: depth, Raw: v, Path: objectPath, Parent: parent, Reliable: stable}
			if v.Index != nil {
				row.Name = fmt.Sprintf("[%d]", *v.Index)
			}
			switch v.Status {
			case observer.NotCaptured:
				row.Value = "未记录"
			case observer.Unavailable:
				row.Value = "无法读取"
			case observer.Observed:
				row.Value = SafeText(v.Value, false)
				if v.Kind == observer.String && v.Value == "" {
					row.Value = "\"\""
				}
				if v.ValueTruncated {
					row.Value += " [截断]"
				}
			default:
				row.Value = "未知"
			}
			if v.Kind == observer.Array {
				n := "?"
				if v.ArrayLength != nil {
					n = fmt.Sprint(*v.ArrayLength)
				}
				row.Detail = fmt.Sprintf("已记录 %d / %s 项", len(v.Children), n)
			}
			if !v.ChildrenComplete && (v.Kind == observer.Array || v.Kind == observer.Struct || len(v.Children) > 0) {
				if row.Detail != "" {
					row.Detail += "; "
				}
				row.Detail += "不完整"
			}
			// String display values are independent of captured character children.
			// ValueTruncated describes the string value; ChildrenComplete describes the tree.
			row.Comparable = match && v.Status == observer.Observed && o.Status == observer.Observed &&
				!v.ValueTruncated && !o.ValueTruncated &&
				(v.Kind == observer.String || (len(v.Children) == 0 && len(o.Children) == 0 &&
					(v.Kind == observer.Scalar || v.Kind == observer.Pointer)))
			row.Changed = row.Comparable && v.Value != o.Value
			row.ReliableChange = strict && row.Changed
			if row.Changed {
				row.Before = SafeText(o.Value, false)
				if v.Kind == observer.String && o.Value == "" {
					row.Before = "\"\""
				}
			}
			row.Expandable = len(v.Children) > 0
			row.Collapsed = collapsed[key]
			if _, ok := collapsed[key]; !ok && collapsed != nil && row.Expandable && (v.Kind == observer.Array || v.Kind == observer.Struct) {
				row.Collapsed = true
			}
			rows = append(rows, row)
			if row.Expandable && !row.Collapsed {
				var oldChildren []observer.Variable
				if match && v.Status == observer.Observed && o.Status == observer.Observed {
					oldChildren = o.Children
				}
				walk(v.Children, oldChildren, key, objectPath, depth+1, strict && v.Status == observer.Observed && o.Status == observer.Observed, stable)
			}
		}
	}
	for i, frame := range s.Frames {
		// The first version shows the active frame; callers can add frame selection later.
		if i != r.Frame {
			continue
		}
		var old []observer.Variable
		reliable := false
		if p != nil && frame.ID != "" && frame.IdentityReliable {
			for _, f := range p.Frames {
				if f.ID == frame.ID && f.IdentityReliable {
					old = f.Variables
					reliable = true
					break
				}
			}
		}
		if !reliable && p != nil && compatibleFrames(s.Frames, p.Frames, i) {
			old = p.Frames[i].Variables
		}
		walk(frame.Variables, old, fmt.Sprintf("frame:%q", frame.ID), "", 0, reliable, frame.ID != "" && frame.IdentityReliable)
	}
	var oldGlobals []observer.Variable
	if p != nil {
		oldGlobals = p.Globals
	}
	walk(s.Globals, oldGlobals, "globals", "", 0, p != nil, true)
	return rows
}
