package tui

import (
	"fmt"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
)

// Bounds is a closed interval of actual array indices, never an array length.
type Bounds struct {
	Lo, Hi int
	Valid  bool
}

func (b *Bounds) Add(i int) {
	if i < 0 {
		return
	}
	if !b.Valid {
		b.Lo, b.Hi, b.Valid = i, i, true
		return
	}
	b.Lo = min(b.Lo, i)
	b.Hi = max(b.Hi, i)
}
func (b Bounds) Contains(i int) bool { return !b.Valid || i >= b.Lo && i <= b.Hi }
func (b Bounds) ExpandWithin(margin int, recorded Bounds) Bounds {
	if !b.Valid || !recorded.Valid {
		return recorded
	}
	// Avoid overflow even for malformed extremely large indices.
	return Bounds{Lo: b.Lo - min(margin, b.Lo-recorded.Lo), Hi: b.Hi + min(margin, recorded.Hi-b.Hi), Valid: true}
}

type FocusRange struct {
	Rows, Cols Bounds
	Automatic  bool
	Boundary   bool
}
type arrayEvidence struct {
	rows, cols, changedRows, changedCols Bounds
	matrix, unreliable                   bool
}

type lineKey struct {
	File string
	Line int
}
type HistoryIndex struct {
	Changes []int
	Objects map[string][]HistoryEntry
	Lines   map[lineKey][]HistoryEntry
	Focus   map[string]FocusRange
}

func capLabel(s string) string {
	r := []rune(s)
	if len(r) > 240 {
		return string(r[:240]) + "…"
	}
	return s
}

// BuildHistoryIndex visits every recorded node once per snapshot, plus its
// bounded ancestors. It stores change/baseline events and bounds, not copies of
// the complete variable trees. Identity remains the engine's responsibility.
func BuildHistoryIndex(run observer.RunResult) *HistoryIndex {
	idx := &HistoryIndex{Objects: map[string][]HistoryEntry{}, Lines: map[lineKey][]HistoryEntry{}, Focus: map[string]FocusRange{}}
	evidence := map[string]*arrayEvidence{}
	lastSeen := map[string]int{}
	r := Replay{Run: run}
	for pos, s := range run.Snapshots {
		r.Position = pos
		var rows []VariableRow
		for frame := 0; frame < max(1, len(s.Frames)); frame++ {
			r.Frame = frame
			for _, row := range r.Rows(nil) {
				if frame > 0 && (row.Parent == "globals" || strings.HasPrefix(row.Key, "globals/")) {
					continue
				}
				rows = append(rows, row)
			}
		}
		byKey := make(map[string]VariableRow, len(rows))
		for _, v := range rows {
			byKey[v.Key] = v
		}
		changedLabels := map[string][]string{}
		var allLabels []string
		anyChange := false
		for _, v := range rows {
			if v.Raw.Kind == observer.Array && v.Reliable {
				if evidence[v.Key] == nil {
					evidence[v.Key] = &arrayEvidence{}
				}
			}
			if v.Changed {
				anyChange = true
				label := capLabel(SafeText(v.Path, false) + ": " + v.Before + " → " + v.Value)
				if len(allLabels) < 6 {
					allLabels = append(allLabels, label)
				}
				for parent := v.Parent; parent != ""; {
					a, ok := byKey[parent]
					if !ok {
						break
					}
					if len(changedLabels[parent]) < 6 {
						changedLabels[parent] = append(changedLabels[parent], label)
					}
					parent = a.Parent
				}
			}
			// A changed field contributes its enclosing array element index.
			// A matrix additionally tracks the next inner array index.
			child := v
			var inner *int
			for parent := v.Parent; parent != ""; {
				a, ok := byKey[parent]
				if !ok {
					break
				}
				if a.Raw.Kind == observer.Array && a.Reliable {
					ev := evidence[a.Key]
					if ev == nil {
						ev = &arrayEvidence{}
						evidence[a.Key] = ev
					}
					if child.Raw.Index == nil || !child.Reliable || !v.Reliable {
						ev.unreliable = true
					}
					if child.Raw.Index != nil {
						ev.rows.Add(*child.Raw.Index)
						if child.Raw.Kind == observer.Array {
							ev.matrix = true
							if inner != nil {
								ev.cols.Add(*inner)
							}
						}
						if v.Changed && child.Reliable {
							ev.changedRows.Add(*child.Raw.Index)
							if child.Raw.Kind == observer.Array && inner != nil {
								ev.changedCols.Add(*inner)
							}
						}
					}
					inner = child.Raw.Index
				}
				child = a
				parent = a.Parent
			}
		}
		for _, v := range rows {
			if !v.Reliable || v.Raw.Status != observer.Observed || v.Raw.ValueTruncated {
				continue
			}
			seen, ok := lastSeen[v.Key]
			entry := HistoryEntry{Position: pos, Location: s.Location}
			switch {
			case !ok || seen != pos-1:
				entry.Text = "观测起点 · " + cellFor(v).Text
			case v.Changed:
				entry.Text = v.Before + " → " + v.Value
				entry.Changed = true
			case len(changedLabels[v.Key]) > 0:
				entry.Text = strings.Join(changedLabels[v.Key], "; ")
				entry.Changed = true
			}
			if entry.Text != "" {
				entry.Text = capLabel(entry.Text)
				idx.Objects[v.Key] = append(idx.Objects[v.Key], entry)
			}
			lastSeen[v.Key] = pos
		}
		if anyChange {
			idx.Changes = append(idx.Changes, pos)
		}
		if s.Location.File != "" && s.Location.Line > 0 {
			key := lineKey{s.Location.File, s.Location.Line}
			label := "暂停于此行"
			if len(allLabels) > 0 {
				label += " · " + strings.Join(allLabels, "; ")
			}
			idx.Lines[key] = append(idx.Lines[key], HistoryEntry{Position: pos, Location: s.Location, Text: capLabel(label), Changed: anyChange})
		}
	}
	for key, ev := range evidence {
		f := FocusRange{Rows: ev.rows, Cols: ev.cols}
		if !ev.unreliable && ev.changedRows.Valid && (!ev.matrix || ev.changedCols.Valid) {
			margin := 2
			if ev.matrix {
				margin = 1
			}
			f.Rows = ev.changedRows.ExpandWithin(margin, ev.rows)
			if ev.matrix {
				f.Cols = ev.changedCols.ExpandWithin(1, ev.cols)
			}
			f.Automatic = true
			f.Boundary = ev.changedRows.Lo == ev.rows.Lo || ev.changedRows.Hi == ev.rows.Hi ||
				ev.matrix && (ev.changedCols.Lo == ev.cols.Lo || ev.changedCols.Hi == ev.cols.Hi)
		}
		idx.Focus[key] = f
	}
	return idx
}
func (r Replay) historyIndex() *HistoryIndex {
	if r.Index != nil {
		return r.Index
	}
	return BuildHistoryIndex(r.Run)
}
func (r Replay) FocusFor(row VariableRow) FocusRange {
	if !row.Reliable {
		return FocusRange{}
	}
	return r.historyIndex().Focus[row.Key]
}
func focusLabel(f FocusRange) string {
	if !f.Automatic || !f.Rows.Valid {
		return ""
	}
	s := fmt.Sprintf("自动关注 %d..%d", f.Rows.Lo, f.Rows.Hi)
	if f.Cols.Valid {
		s += fmt.Sprintf(" × %d..%d", f.Cols.Lo, f.Cols.Hi)
	}
	if f.Boundary {
		s += " · 变化触及采集边界"
	}
	return s
}
