package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/Annouimo/c-observer/internal/observer"
)

// Fixture is a development-only record. It is never presented as a live run.
type Fixture struct {
	Synthetic bool               `json:"synthetic"`
	Stdin string                 `json:"stdin"`
	ExpectedStdout *string       `json:"expected_stdout"`
	Run observer.RunResult       `json:"run"`
}

func DecodeFixture(r io.Reader) (Fixture, error) {
	var f Fixture
	if err := json.NewDecoder(r).Decode(&f); err != nil { return f, err }
	if !f.Synthetic || f.Run.Session.Backend != "synthetic" {
		return f, fmt.Errorf("preview requires an explicitly synthetic fixture")
	}
	if err := ValidateRun(f.Run); err != nil { return f, err }
	return f, nil
}

// ValidateRun protects replay from malformed offsets, including non-monotonic ones.
func ValidateRun(run observer.RunResult) error {
	if run.Reason == "" { return fmt.Errorf("missing run result") }
	prev := observer.OutputOffsets{}
	for i, s := range run.Snapshots {
		o := s.OutputOffsets
		if s.ID != i || o.StdoutBytes < prev.StdoutBytes || o.StderrBytes < prev.StderrBytes ||
			o.StdoutBytes > len(run.Output.Stdout) || o.StderrBytes > len(run.Output.Stderr) {
			return fmt.Errorf("invalid snapshot or output offset at %d", i)
		}
		prev = o
	}
	return nil
}

// Replay position n is a synthetic END screen, not a debugger snapshot.
type Replay struct {
	Run observer.RunResult
	Position int
}

func (r *Replay) Move(delta int) { r.Seek(r.Position + delta) }
func (r *Replay) Seek(pos int) { r.Position = max(0, min(pos, len(r.Run.Snapshots))) }
func (r Replay) End() bool { return r.Position == len(r.Run.Snapshots) }
func (r Replay) Current() *observer.Snapshot {
	if r.Position < 0 || r.Position >= len(r.Run.Snapshots) { return nil }
	return &r.Run.Snapshots[r.Position]
}
func (r Replay) Previous() *observer.Snapshot {
	if r.Position <= 0 || r.End() { return nil }
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
		case r == '\n' && multiline: b.WriteRune(r)
		case r == '\t': b.WriteString("    ")
		case r == '\r': b.WriteString("\\r")
		case r == '\n': b.WriteString("\\n")
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r):
			fmt.Fprintf(&b, "\\u%04x", r)
		default: b.WriteRune(r)
		}
	}
	return b.String()
}

type VariableRow struct {
	Key, Name, Type, Value, Detail, Before string
	Depth int
	Changed, Expandable, Collapsed bool
	Reliable bool
	Path string
	Parent string
	Raw observer.Variable
}

// Rows compares only against the chronologically previous snapshot. Unreliable
// frames or ancestors suppress change claims, even if display names match.
func (r Replay) Rows(collapsed map[string]bool) []VariableRow {
	s := r.Current()
	if s == nil { return nil }
	p := r.Previous()
	var rows []VariableRow
	var walk func([]observer.Variable, []observer.Variable, string, string, int, bool, bool)
	walk = func(vars, old []observer.Variable, parent, path string, depth int, reliable, identity bool) {
		oldByID := make(map[string]observer.Variable)
		for _, v := range old {
			if v.ID != "" && v.IdentityReliable { oldByID[v.ID] = v }
		}
		for i, v := range vars {
			key := fmt.Sprintf("%s/%d:%s", parent, i, v.ID)
			stable := identity && v.ID!="" && v.IdentityReliable
			if stable { key = fmt.Sprintf("%s/%q",parent,v.ID) }
			objectPath := v.Name
			if path!="" {
				if v.Index!=nil {objectPath=fmt.Sprintf("%s[%d]",path,*v.Index)} else {objectPath=path+"."+v.Name}
			}
			o, ok := oldByID[v.ID]
			match := reliable && v.ID != "" && v.IdentityReliable && ok && o.Kind == v.Kind && o.Type == v.Type
			row := VariableRow{Key:key, Name:SafeText(v.Name,false), Type:SafeText(v.Type,false), Depth:depth, Raw:v, Path:objectPath, Parent:parent, Reliable:stable}
			if v.Index != nil { row.Name = fmt.Sprintf("[%d]", *v.Index) }
			switch v.Status {
			case observer.NotCaptured: row.Value = "未记录"
			case observer.Unavailable: row.Value = "无法读取"
			case observer.Observed:
				row.Value = SafeText(v.Value,false)
				if v.Kind == observer.String && v.Value == "" { row.Value = "\"\"" }
				if v.ValueTruncated { row.Value += " [截断]" }
			default: row.Value = "未知"
			}
			if v.Kind == observer.Array {
				n := "?"
				if v.ArrayLength != nil { n = fmt.Sprint(*v.ArrayLength) }
				row.Detail = fmt.Sprintf("已记录 %d / %s 项", len(v.Children), n)
			}
			if !v.ChildrenComplete && (v.Kind == observer.Array || v.Kind == observer.Struct || len(v.Children)>0) {
				if row.Detail != "" { row.Detail += "; " }
				row.Detail += "不完整"
			}
			row.Changed = match && v.Status == observer.Observed && o.Status == observer.Observed &&
				!v.ValueTruncated && !o.ValueTruncated && len(v.Children)==0 && len(o.Children)==0 &&
				(v.Kind == observer.Scalar || v.Kind == observer.String || v.Kind == observer.Pointer) && v.Value != o.Value
			if row.Changed { row.Before = SafeText(o.Value,false) }
			row.Expandable = len(v.Children)>0
			row.Collapsed = collapsed[key]
			rows = append(rows,row)
			if row.Expandable && !row.Collapsed {
				var oldChildren []observer.Variable
				if match { oldChildren=o.Children }
				walk(v.Children,oldChildren,key,objectPath,depth+1,match && v.Status==observer.Observed && o.Status==observer.Observed,stable)
			}
		}
	}
	for i, frame := range s.Frames {
		// The first version shows the active frame; callers can add frame selection later.
		if i>0 { break }
		var old []observer.Variable
		reliable := false
		if p!=nil && frame.ID!="" && frame.IdentityReliable {
			for _, f := range p.Frames {
				if f.ID==frame.ID && f.IdentityReliable { old=f.Variables; reliable=true; break }
			}
		}
		walk(frame.Variables,old,"frame:"+frame.ID,"",0,reliable,frame.ID!="" && frame.IdentityReliable)
	}
	var oldGlobals []observer.Variable
	if p!=nil { oldGlobals=p.Globals }
	walk(s.Globals,oldGlobals,"globals","",0,p!=nil,true)
	return rows
}
