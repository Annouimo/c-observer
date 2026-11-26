package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
)

func indexLabel(s string) (int, bool) {
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return 0, false
	}
	i, err := strconv.Atoi(s[1 : len(s)-1])
	return i, err == nil && i >= 0
}

// focusTable never allocates an array by its declared capacity. Small missing
// intervals get explicit placeholders; large gaps are compressed into one row.
func focusTable(t DataTable, f FocusRange) DataTable {
	if !f.Rows.Valid || len(t.Headers) == 0 {
		return t
	}
	columns := []int{0}
	for i, h := range t.Headers[1:] {
		n, ok := indexLabel(h)
		if !f.Cols.Valid || !ok || f.Cols.Contains(n) {
			columns = append(columns, i+1)
		}
	}
	out := DataTable{}
	for _, i := range columns {
		out.Headers = append(out.Headers, t.Headers[i])
	}
	if f.Cols.Valid && f.Cols.Hi-f.Cols.Lo <= 32 && t.Headers[0] == "行 / 列" {
		out.Headers = []string{t.Headers[0]}
		columns = []int{0}
		for i := f.Cols.Lo; ; i++ {
			label := fmt.Sprintf("[%d]", i)
			out.Headers = append(out.Headers, label)
			col := -1
			for j, h := range t.Headers {
				if h == label {
					col = j
					break
				}
			}
			columns = append(columns, col)
			if i == f.Cols.Hi {
				break
			}
		}
	}
	byIndex := map[int][]Cell{}
	for _, r := range t.Rows {
		if len(r) == 0 {
			continue
		}
		i, ok := indexLabel(r[0].Text)
		if !ok {
			return t
		}
		if f.Rows.Contains(i) {
			line := []Cell{}
			for _, j := range columns {
				if j >= 0 && j < len(r) {
					line = append(line, r[j])
				} else {
					line = append(line, Cell{Text: "未记录"})
				}
			}
			byIndex[i] = line
		}
	}
	missing := func(label string) []Cell {
		line := []Cell{{Text: label}}
		for range out.Headers[1:] {
			line = append(line, Cell{Text: "未记录"})
		}
		return line
	}
	if f.Rows.Hi-f.Rows.Lo <= 64 {
		for i := f.Rows.Lo; ; i++ {
			if line, ok := byIndex[i]; ok {
				out.Rows = append(out.Rows, line)
			} else {
				out.Rows = append(out.Rows, missing(fmt.Sprintf("[%d]", i)))
			}
			if i == f.Rows.Hi {
				break
			}
		}
	} else {
		// A sparse page should not turn into millions of synthetic rows.
		var indices []int
		for i := range byIndex {
			indices = append(indices, i)
		}
		sort.Ints(indices)
		next := f.Rows.Lo
		for _, i := range indices {
			if i > next {
				out.Rows = append(out.Rows, missing(fmt.Sprintf("[%d..%d]", next, i-1)))
			}
			out.Rows = append(out.Rows, byIndex[i])
			if i == f.Rows.Hi {
				return out
			}
			next = i + 1
		}
		if next <= f.Rows.Hi {
			out.Rows = append(out.Rows, missing(fmt.Sprintf("[%d..%d]", next, f.Rows.Hi)))
		}
	}
	return out
}
func (m Model) displayFocus(row VariableRow) FocusRange {
	if m.fullArrays[row.Key] {
		return FocusRange{}
	}
	if f, ok := m.manualRanges[row.Key]; ok {
		return f
	}
	if row.Raw.Kind != observer.Array || !row.Reliable || m.replay.Index == nil {
		return FocusRange{}
	}
	f := m.replay.FocusFor(row)
	if !f.Automatic {
		return FocusRange{}
	}
	return f
}

func (m Model) dataTable(row VariableRow, rows []VariableRow) DataTable {
	t := TableFor(row, rows)
	f := m.displayFocus(row)
	if f.Rows.Valid {
		return focusTable(t, f)
	}
	return t
}

func (m Model) rangeLabel(row VariableRow) string {
	f := m.displayFocus(row)
	if f.Automatic {
		return focusLabel(f)
	}
	if f.Rows.Valid {
		label := fmt.Sprintf("手动展示 %d..%d", f.Rows.Lo, f.Rows.Hi)
		if f.Cols.Valid {
			label += fmt.Sprintf(" × %d..%d", f.Cols.Lo, f.Cols.Hi)
		}
		return label
	}
	if m.fullArrays[row.Key] {
		return "全部已采集"
	}
	return ""
}
