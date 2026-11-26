package tui

import (
	"fmt"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
	"github.com/charmbracelet/lipgloss"
)

// executionContext uses the origin of the observed transition, not the
// destination pause. Until the engine supplies stronger evidence, all
// transitions are intervals, never asserted single-line execution events.
func (m Model) executionContext(w int) []string {
	step := m.replay.Step()
	if !step.HasInterval() {
		loc := m.replay.ViewedLocation()
		return []string{
			muted.Render(clip("初始观察 · "+SafeText(fmt.Sprintf("%s:%d", loc.File, loc.Line), false), w)),
			muted.Render(clip("尚无前一份记录，不能归因于当前行。", w)),
		}
	}
	loc := step.Origin
	lines := []string{muted.Render(clip(SafeText(fmt.Sprintf("%s · %s:%d", loc.Function, loc.File, loc.Line), false), w))}
	source := strings.Split(sourceFor(m.replay.Run, loc.File), "\n")
	if loc.File != "" && loc.Line > 0 && loc.Line <= len(source) && sourceFor(m.replay.Run, loc.File) != "" {
		lines = append(lines, accent.Render(clip(fmt.Sprintf("▸ %d  %s", loc.Line, strings.TrimSpace(SafeText(source[loc.Line-1], false))), w)))
	} else {
		lines = append(lines, muted.Render(clip("区间起点源码不可映射", w)))
	}
	var labels []string
	count := 0
	for _, row := range m.replay.Rows(nil) {
		if row.Changed {
			count++
			if len(labels) < 2 {
				labels = append(labels, SafeText(row.Path, false)+": "+row.Before+" → "+row.Value)
			}
		}
	}
	summary := "无已观测可靠变化"
	if count > 0 {
		summary = strings.Join(labels, "; ")
		if count > len(labels) {
			summary += fmt.Sprintf(" 等 %d 项", count)
		}
	}
	lines = append(lines, clip("区间变化（单行归因未确认）: "+summary, w))
	return lines
}

type observationBlock struct {
	row   VariableRow
	lines []string
}

func previewCell(c Cell, w int) string {
	value := c.Text
	if c.Current != "" {
		value = c.Current
	}
	if c.Changed {
		value = "*" + value
	}
	value = pad(clip(value, w), w)
	if c.Changed {
		return changed.Render(value)
	}
	return value
}

// previewBlock always returns a visible object heading. Each subsequent
// line is optional; it can be removed without removing the object itself.
func (m Model) previewBlock(row VariableRow, all []VariableRow, w int, active bool) observationBlock {
	prefix := " "
	if active {
		prefix = ">"
	}
	title := prefix + row.Name
	aggregate := row.Raw.Kind == observer.Array || row.Raw.Kind == observer.Struct
	if !aggregate || row.Raw.Status != observer.Observed {
		value := row.Value
		if row.Changed {
			value = "*" + value
		}
		line := clip(title+" = "+value, w)
		if row.Changed {
			line = changed.Render(line)
		}
		return observationBlock{row: row, lines: []string{line}}
	}
	title += " · " + row.Detail
	if row.Detail == "" {
		title += fmt.Sprintf("%d 个已记录字段", len(row.Raw.Children))
	}
	b := observationBlock{row: row, lines: []string{clip(title, w)}}
	if label := m.rangeLabel(row); label != "" {
		b.lines = append(b.lines, muted.Render(clip(label, w)))
	}
	t := m.dataTable(row, all)
	if len(t.Rows) == 0 || len(t.Headers) < 2 {
		return b
	}
	// Fixed cell widths keep ordinary value changes from moving columns.
	// Long values can be inspected in full in the existing detail screen.
	if row.Raw.Kind == observer.Array && len(t.Headers) == 3 && t.Headers[1] == "值" {
		for _, r := range t.Rows {
			if len(r) >= 2 {
				b.lines = append(b.lines, pad(clip(r[0].Text, 12), 12)+" "+previewCell(r[1], max(1, w-13)))
			}
		}
		return b
	}
	indexW := min(12, max(6, w/4))
	cellW := 9
	n := max(1, min(len(t.Headers)-1, (w-indexW)/(cellW+1)))
	header := pad(clip(t.Headers[0], indexW), indexW)
	for i := 1; i <= n; i++ {
		header += " " + pad(clip(t.Headers[i], cellW), cellW)
	}
	if n < len(t.Headers)-1 {
		header = clip(header, w-1) + "…"
	}
	b.lines = append(b.lines, muted.Render(header))
	for _, r := range t.Rows {
		if len(r) == 0 {
			continue
		}
		line := pad(clip(r[0].Text, indexW), indexW)
		for i := 1; i <= n && i < len(r); i++ {
			line += " " + previewCell(r[i], cellW)
		}
		b.lines = append(b.lines, line)
	}
	return b
}

// dataFirstPanel allocates a heading to each root before spending the
// remaining height on previews. Horizontal packing is deterministic for
// the same object set and terminal width.
func (m Model) dataFirstPanel(w, h int) string {
	all := m.replay.Rows(nil)
	var roots []VariableRow
	for _, row := range all {
		if row.Depth == 0 {
			roots = append(roots, row)
		}
	}
	if len(roots) == 0 {
		return fixed([]string{"无已记录变量。"}, w, h)
	}
	if h <= 0 {
		return ""
	}
	cols := min(len(roots), max(1, w/25))
	colW := (w - cols + 1) / cols
	groups := (len(roots) + cols - 1) / cols
	selectedRow, _ := m.selectedVariable()
	blocks := make([]observationBlock, len(roots))
	for i, row := range roots {
		blocks[i] = m.previewBlock(row, all, colW, selectedRow.Key == row.Key || strings.HasPrefix(selectedRow.Key, row.Key+"/"))
	}
	available := max(1, h-1)
	visible := min(groups, available)
	start := 0
	if groups > visible {
		for i, b := range blocks {
			if selectedRow.Key == b.row.Key || strings.HasPrefix(selectedRow.Key, b.row.Key+"/") {
				start = max(0, min(i/cols-visible+1, groups-visible))
				break
			}
		}
	}
	// First reserve all group headings; then grow the previews one row per
	// group per pass. No early large object can starve later headings.
	heights := make([]int, visible)
	for i := range heights {
		heights[i] = 1
	}
	remaining := available - visible
	for remaining > 0 {
		grew := false
		for g := range heights {
			need := 1
			for i := (g + start) * cols; i < min(len(blocks), (g+start+1)*cols); i++ {
				need = max(need, len(blocks[i].lines))
			}
			if heights[g] < need {
				heights[g]++
				remaining--
				grew = true
			}
			if remaining == 0 {
				break
			}
		}
		if !grew {
			break
		}
	}
	label := fmt.Sprintf("数据 · %d 个对象", len(roots))
	if groups > visible {
		label += " · ↑/↓ 浏览其余对象"
	}
	if m.replay.Frame > 0 {
		label += fmt.Sprintf(" · 所选帧 #%d", m.replay.Frame)
	}
	lines := []string{muted.Render(clip(label, w))}
	for g, height := range heights {
		for line := 0; line < height; line++ {
			var cells []string
			for i := (g + start) * cols; i < min(len(blocks), (g+start+1)*cols); i++ {
				b := blocks[i]
				cell := ""
				if line < len(b.lines) {
					cell = b.lines[line]
					if line == height-1 && len(b.lines) > height {
						// A preview is shortened, not an unrecorded value.
						if lipgloss.Width(cell) < colW-1 {
							cell += " …"
						} else {
							cell = muted.Render(clip(b.row.Name+" · 详情见 D", colW))
						}
					}
				}
				cells = append(cells, pad(cell, colW))
			}
			lines = append(lines, strings.Join(cells, " "))
		}
	}
	return fixed(lines, w, h)
}
