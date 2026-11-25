package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Annouimo/c-observer/internal/observer"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
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

// arrayStrip uses current values with real indices and a per-cell change
// marker; it wraps to terminal width and never fills omitted data with zero.
func arrayStrip(t DataTable, w int) []string {
	if len(t.Rows) == 0 {
		return nil
	}
	var out []string
	var labels, values []string
	used := 0
	flush := func() {
		if len(labels) > 0 {
			out = append(out, muted.Render(strings.Join(labels, " ")), strings.Join(values, " "))
		}
		labels, values = nil, nil
		used = 0
	}
	for _, r := range t.Rows {
		if len(r) < 2 {
			continue
		}
		value := r[1].Text
		if r[1].Current != "" {
			value = r[1].Current
		}
		if r[1].Changed {
			value = "*" + value
		}
		width := min(max(5, runewidth.StringWidth(r[0].Text), runewidth.StringWidth(value)), min(w, 24))
		if used+width+1 > w && used > 0 {
			flush()
		}
		labels = append(labels, pad(clip(r[0].Text, width), width))
		cell := pad(clip(value, width), width)
		if r[1].Changed {
			cell = changed.Render(cell)
		}
		values = append(values, cell)
		used += width + 1
	}
	flush()
	return out
}

func (m Model) dataPreview(row VariableRow, all []VariableRow, w int) []string {
	if row.Raw.Status != observer.Observed || !row.Expandable || row.Depth > 0 {
		return nil
	}
	if row.Raw.Kind != observer.Array && row.Raw.Kind != observer.Struct {
		return nil
	}
	t := m.dataTable(row, all)
	if len(t.Rows) > 8 || len(t.Headers) > 6 {
		return nil
	}
	if row.Raw.Kind == observer.Array && len(t.Headers) == 3 && t.Headers[1] == "值" {
		lines := arrayStrip(t, w)
		if len(lines) <= 4 {
			return lines
		}
		return nil
	}
	// Tables are previewed only if all recorded columns fit. Details page is
	// available for wide or nested values.
	if len(t.Rows) > 4 {
		return nil
	}
	need := 0
	for i, h := range t.Headers {
		cw := max(8, runewidth.StringWidth(h))
		for _, r := range t.Rows {
			if i < len(r) {
				cw = max(cw, runewidth.StringWidth(r[i].Text)+2)
			}
		}
		need += min(cw, 28) + 3
	}
	if need-3 > w {
		return nil
	}
	return renderTable(t, w, 2+len(t.Rows), 0, 0)
}

func (m Model) variablePanel(w, h int) string {
	s := m.replay.Current()
	lines := []string{head(" 数据", m.focus == 1, w)}
	if s == nil {
		return fixed(append(lines, muted.Render("没有当前变量状态。")), w, h)
	}
	label := "当前栈帧"
	if m.replay.Frame < len(s.Frames) {
		f := s.Frames[m.replay.Frame]
		label = fmt.Sprintf("%s · 栈帧 %d/%d", SafeText(f.Location.Function, false), m.replay.Frame+1, len(s.Frames))
		if !f.VariablesComplete {
			label += " · 部分变量"
		}
	}
	if len(s.Globals) > 0 {
		label += " · 全局"
	}
	if !s.GlobalsComplete && m.replay.Run.Session.Capabilities.Globals {
		label += " · 全局不完整"
	}
	lines = append(lines, muted.Render(clip(label, w)))
	rows := m.replay.Rows(m.collapsed)
	all := m.replay.Rows(nil)
	if len(rows) == 0 {
		return fixed(append(lines, muted.Render("无已记录变量。")), w, h)
	}
	// Each variable is a block; selection is by object, not rendered line.
	type block struct{ lines []string }
	blocks := []block{}
	lastScope := ""
	for i, r := range rows {
		mark := " "
		if m.focus == 1 && i == m.varCursor {
			mark = ">"
		}
		prefix := mark + strings.Repeat("  ", min(r.Depth, 5)) + r.Name
		value := r.Value
		if r.Changed {
			value = "* " + r.Before + " → " + r.Value
		}
		if r.Detail != "" {
			if value != "" {
				value += " · "
			}
			value += r.Detail
		}
		hasChange := r.Changed
		if r.Collapsed {
			count := 0
			for _, v := range all {
				if v.Changed && strings.HasPrefix(v.Key, r.Key+"/") {
					count++
				}
			}
			if count > 0 {
				hasChange = true
				value = fmt.Sprintf("* %d 项变化 · %s", count, value)
			}
		}
		aggregate := r.Raw.Kind == observer.Array || r.Raw.Kind == observer.Struct
		if aggregate {
			prefix += "  " + r.Type
		}
		nameW := max(8, w*28/100)
		typeW := max(7, min(14, w*24/100))
		line := pad(clip(prefix, nameW), nameW) + " " + pad(clip(r.Type, typeW), typeW) + " " + clip(value, max(0, w-nameW-typeW-2))
		if aggregate {
			line = clip(prefix+" · "+value, w)
		}
		if hasChange {
			line = changed.Render(line)
		}
		if m.focus == 1 && i == m.varCursor {
			line = selected.Render(line)
		}
		b := block{}
		if r.Depth == 0 {
			scope := scopeLabel(r)
			if scope != lastScope {
				b.lines = append(b.lines, muted.Render(clip("  "+scope+" · 名称 / 类型 / 值", w)))
				lastScope = scope
			}
		}
		b.lines = append(b.lines, line)
		if aggregate && r.Collapsed && r.Depth == 0 {
			if rangeText := m.rangeLabel(r); rangeText != "" {
				b.lines = append(b.lines, muted.Render(clip("  "+rangeText, w)))
			}
			b.lines = append(b.lines, m.dataPreview(r, all, w)...)
		}
		blocks = append(blocks, b)
	}
	cursor := max(0, min(m.varCursor, len(blocks)-1))
	avail := max(1, h-2)
	start := cursor
	used := len(blocks[cursor].lines)
	for start > 0 && used+len(blocks[start-1].lines) <= avail {
		start--
		used += len(blocks[start].lines)
	}
	for i := start; i < len(blocks) && len(lines) < h; i++ {
		for _, line := range blocks[i].lines {
			if len(lines) >= h {
				break
			}
			lines = append(lines, line)
		}
	}
	// lipgloss measures ANSI strings by terminal cells.
	for i, line := range lines {
		if lipgloss.Width(line) > w {
			lines[i] = clip(SafeText(line, false), w)
		}
	}
	return fixed(lines, w, h)
}

func scopeLabel(r VariableRow) string {
	switch r.Raw.Scope {
	case observer.ParameterScope:
		return "参数"
	case observer.GlobalScope:
		return "全局变量"
	case observer.FileStaticScope:
		return "文件静态变量"
	case observer.FunctionStaticScope:
		return "函数静态变量"
	case observer.LocalScope:
		return "局部变量"
	}
	if r.Parent == "globals" {
		return "全局变量"
	}
	return "当前帧变量"
}
