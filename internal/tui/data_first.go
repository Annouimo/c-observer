package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
	"github.com/mattn/go-runewidth"
)

// executionContext uses the origin of the observed transition, not the
// destination pause. Until the engine supplies stronger evidence, all
// transitions are intervals, never asserted single-line execution events.
func (m Model) executionContext(w int) []string {
	step := m.replay.Step()
	loc := step.Origin
	if !step.HasInterval() {
		loc = m.replay.ViewedLocation()
	}
	title := fmt.Sprintf("%s · %s:%d", loc.Function, filepath.Base(loc.File), loc.Line)
	if !step.HasInterval() {
		title = fmt.Sprintf("初始观察 · %s:%d", filepath.Base(loc.File), loc.Line)
	}
	lines := []string{muted.Render(clip(SafeText(title, false), w))}
	src := sourceFor(m.replay.Run, loc.File)
	source := strings.Split(src, "\n")
	radius := 0
	if m.height >= 24 {
		radius = 1
	}
	for i := loc.Line - radius; i <= loc.Line+radius; i++ {
		switch {
		case loc.File != "" && src != "" && i > 0 && i <= len(source):
			text := strings.TrimSpace(SafeText(source[i-1], false))
			if i == loc.Line {
				lines = append(lines, accent.Render(clip(fmt.Sprintf("▸ %d  %s", i, text), w)))
			} else {
				lines = append(lines, muted.Render(clip(fmt.Sprintf("  %d  %s", i, text), w)))
			}
		case i == loc.Line:
			lines = append(lines, muted.Render(clip("源码位置不可映射", w)))
		default:
			lines = append(lines, "")
		}
	}
	if !step.HasInterval() {
		return append(lines, muted.Render(clip("尚无前一份记录，不能归因于当前行。", w)))
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
	const prefix = "区间变化（单行归因未确认）: "
	summary := "无已观测可靠变化"
	for n := len(labels); n > 0; n-- {
		candidate := strings.Join(labels[:n], "; ")
		if n < count {
			candidate += fmt.Sprintf(" 等 %d 项", count)
		}
		if cellWidth(prefix+candidate) <= w {
			summary = candidate
			break
		}
		summary = fmt.Sprintf("%d 项 · H 查看历史", count)
	}
	lines = append(lines, clip(prefix+summary, w))
	return lines
}

var layoutIndex = regexp.MustCompile(`\[[0-9]+\]`)

func layoutKey(path string) string { return layoutIndex.ReplaceAllString(path, "[]") }
func cellWidth(text string) int    { return runewidth.StringWidth(text) }

func rootRows(all []VariableRow) []VariableRow {
	var roots []VariableRow
	for _, row := range all {
		if row.Depth == 0 {
			roots = append(roots, row)
		}
	}
	return roots
}

type previewGroup struct {
	title    string
	table    DataTable
	widths   []int
	sequence bool
	cropped  bool
}

type observationBlock struct {
	rangeText            string
	title                string
	groups               []previewGroup
	minWidth, idealWidth int
	aggregate            bool
}

func aggregate(row VariableRow) bool {
	return row.Raw.Kind == observer.Array || row.Raw.Kind == observer.Struct ||
		row.Raw.Kind == observer.String && len(row.Raw.Children) > 0
}

func (m Model) measuredValue(path, value string) int {
	width := cellWidth(value)
	if idx := m.replay.Index; idx != nil {
		width = max(width, idx.PreviewWidth[layoutKey(path)])
	}
	return width + 1 // Reserve the change marker on every step.
}

func (m Model) previewGroup(row VariableRow, all []VariableRow, title string) previewGroup {
	table := m.dataTable(row, all)
	g := previewGroup{title: title, table: table, widths: make([]int, len(table.Headers))}
	focus := m.displayFocus(row)
	if focus.Rows.Valid {
		for _, child := range row.Raw.Children {
			if child.Index == nil {
				continue
			}
			if !focus.Rows.Contains(*child.Index) {
				g.cropped = true
				break
			}
			for _, cell := range child.Children {
				if cell.Index != nil && !focus.Cols.Contains(*cell.Index) {
					g.cropped = true
					break
				}
			}
		}
	}
	g.sequence = len(table.Headers) == 3 && table.Headers[0] == "下标" && table.Headers[1] == "值"
	if g.sequence {
		g.widths = g.widths[:2] // Types remain in details.
	}
	for col := range g.widths {
		g.widths[col] = cellWidth(table.Headers[col])
		for _, cells := range table.Rows {
			if col >= len(cells) {
				continue
			}
			cell := cells[col]
			value := currentText(cell)
			width := cellWidth(value)
			if col > 0 {
				key := row.Path
				switch {
				case g.sequence || table.Headers[0] == "行 / 列":
					key += "[]"
					if !g.sequence {
						key += "[]"
					}
				case table.Headers[0] == "下标":
					key += "[]." + table.Headers[col]
				case table.Headers[0] == "成员":
					key += "." + cells[0].Text
				}
				width = m.measuredValue(key, value)
			}
			g.widths[col] = max(g.widths[col], width)
		}
	}
	return g
}

func currentText(c Cell) string {
	if c.Current != "" {
		return c.Current
	}
	return c.Text
}

func (m Model) makeBlock(row VariableRow, all []VariableRow, w int, active bool) observationBlock {
	prefix := " "
	if active {
		prefix = ">"
	}
	name := prefix + row.Name
	if row.Parent == "globals" {
		name += "⁺"
	}
	b := observationBlock{title: name}
	if !aggregate(row) || row.Raw.Status != observer.Observed {
		value := row.Value
		if row.Changed {
			value = "*" + value
		}
		b.title += " = " + value
		b.idealWidth = min(w, max(10, cellWidth(name)+3+m.measuredValue(row.Path, row.Value)))
		b.minWidth = min(b.idealWidth, 30)
		return b
	}
	b.aggregate = true
	b.rangeText = m.rangeLabel(row)
	b.title = name + fmt.Sprintf(" · %d", len(row.Raw.Children))
	if row.Raw.ArrayLength != nil {
		b.title += fmt.Sprintf("/%d", *row.Raw.ArrayLength)
	}
	if !row.Raw.ChildrenComplete {
		b.title += " · 缺采"
	}
	b.minWidth = min(w, max(16, min(26, cellWidth(b.title))))
	b.idealWidth = min(w, max(b.minWidth, cellWidth(b.title)))
	if len(row.Raw.Children) == 0 {
		return b
	}
	group := m.previewGroup(row, all, "")
	if row.Raw.Kind == observer.Struct {
		// Structured children get their own compact preview; retain scalar siblings.
		var simple [][]Cell
		for _, child := range childRows(row, all) {
			if aggregate(child) && child.Raw.Status == observer.Observed && len(child.Raw.Children) > 0 {
				b.groups = append(b.groups, m.previewGroup(child, all, child.Name))
			} else {
				simple = append(simple, []Cell{{Text: child.Name}, cellFor(child)})
			}
		}
		group.table.Rows = simple
		group.widths = group.widths[:min(2, len(group.widths))]
	}
	if len(group.table.Rows) != 0 {
		b.groups = append([]previewGroup{group}, b.groups...)
	}
	for _, g := range b.groups {
		natural := 0
		for _, width := range g.widths {
			natural += width + 2
		}
		if g.sequence {
			if g.widths[1] <= 11 {
				natural = (max(g.widths[0], g.widths[1]) + 1) * min(len(g.table.Rows), 6)
			} else {
				b.minWidth = min(w, max(b.minWidth, min(36, natural-2)))
			}
		}
		b.idealWidth = min(w, max(b.idealWidth, natural-2))
	}
	return b
}

func previewCell(c Cell, width int) string {
	text := currentText(c)
	if c.Changed {
		text = "*" + text
	}
	text = pad(clip(text, width), width)
	if c.Changed {
		return changed.Render(text)
	}
	return text
}

type previewLine struct {
	text  string
	cells int
}

func (g previewGroup) render(w int) ([]previewLine, bool) {
	var lines []previewLine
	if g.title != "" {
		lines = append(lines, previewLine{text: muted.Render(clip("  "+g.title, w))})
	}
	if len(g.widths) < 2 {
		return lines, false
	}
	if g.sequence && g.widths[1] <= 11 {
		cell := min(w, max(g.widths[0], g.widths[1]))
		perLine := max(1, (w+1)/(cell+1))
		for start := 0; start < len(g.table.Rows); start += perLine {
			var keys, values []string
			for _, row := range g.table.Rows[start:min(start+perLine, len(g.table.Rows))] {
				keys = append(keys, pad(clip(row[0].Text, cell), cell))
				values = append(values, previewCell(row[1], cell))
			}
			lines = append(lines, previewLine{text: muted.Render(strings.Join(keys, " "))}, previewLine{text: strings.Join(values, " "), cells: len(values)})
		}
		return lines, g.cropped
	}
	indexWidth := min(g.widths[0], max(5, w/3))
	widths := []int{indexWidth}
	used := indexWidth
	hidden := g.cropped
	for _, width := range g.widths[1:] {
		available := w - used - 2
		if available < 3 || len(widths) > 1 && width > available {
			hidden = true
			break
		}
		widths = append(widths, min(width, available))
		used += widths[len(widths)-1] + 2
		hidden = hidden || width > available
	}
	draw := func(row []Cell) string {
		var parts []string
		for i, width := range widths {
			if i >= len(row) {
				break
			}
			parts = append(parts, previewCell(row[i], width))
		}
		return strings.Join(parts, "  ")
	}
	if !g.sequence && g.table.Headers[0] != "成员" {
		var header []Cell
		for _, label := range g.table.Headers {
			if label == "行 / 列" {
				label = "行╲列"
			}
			header = append(header, Cell{Text: label})
		}
		lines = append(lines, previewLine{text: muted.Render(draw(header))})
	}
	for _, row := range g.table.Rows {
		lines = append(lines, previewLine{text: draw(row), cells: max(0, min(len(row), len(widths))-1)})
	}
	return lines, hidden
}

type observationShelf struct {
	indices            []int
	widths             []int
	lines              [][]previewLine
	height, fullHeight int
}

func packObservations(blocks []observationBlock, w, widthClass int) []observationShelf {
	var shelves []observationShelf
	used := 0
	for i, b := range blocks {
		width := b.minWidth + (b.idealWidth-b.minWidth)*widthClass/3
		if len(shelves) == 0 || used+2+width > w {
			shelves = append(shelves, observationShelf{height: 1, fullHeight: 1})
			used = 0
		}
		shelf := &shelves[len(shelves)-1]
		if len(shelf.indices) != 0 {
			used += 2
		}
		shelf.indices = append(shelf.indices, i)
		shelf.widths = append(shelf.widths, width)
		used += width
	}
	if widthClass < 3 {
		for i := range shelves {
			s := &shelves[i]
			free := w - 2*(len(s.indices)-1)
			for _, width := range s.widths {
				free -= width
			}
			// Grow by demand, so a matrix gets more space than an integer label.
			for free > 0 {
				grow := -1
				maxGap := 0
				for j, index := range s.indices {
					gap := blocks[index].idealWidth - s.widths[j]
					if gap > maxGap {
						maxGap, grow = gap, j
					}
				}
				if grow < 0 {
					break
				}
				s.widths[grow]++
				free--
			}
		}
	}
	return shelves
}

func (m Model) dataFirstPanel(w, h int) string {
	if h <= 0 {
		return ""
	}
	all := m.replay.Rows(nil)
	roots := rootRows(all)
	if len(roots) == 0 {
		return fixed([]string{"无已记录变量。"}, w, h)
	}
	blocks := make([]observationBlock, len(roots))
	for i, row := range roots {
		blocks[i] = m.makeBlock(row, all, w, i == m.varCursor)
	}
	budget := max(1, h-2)
	var view []observationShelf
	var hiddenColumns, offscreen bool
	bestRoots, bestCells := -1, -1
	for _, widthClass := range []int{3, 2, 1, 0} {
		shelves := packObservations(blocks, w, widthClass)
		horizontal := false
		for i := range shelves {
			shelf := &shelves[i]
			for j, index := range shelf.indices {
				b := blocks[index]
				width := shelf.widths[j]
				title := clip(b.title, width)
				if index == m.varCursor {
					title = selected.Render(title)
				}
				if !b.aggregate && roots[index].Changed {
					title = changed.Render(title)
				}
				lines := []previewLine{{text: title}}
				if b.rangeText != "" {
					label, _, _ := strings.Cut(b.rangeText, " · ")
					lines = append(lines, previewLine{text: muted.Render(clip(label, width))})
				}
				horizontal = horizontal || cellWidth(b.title) > width
				for _, g := range b.groups {
					extra, hidden := g.render(width)
					horizontal = horizontal || hidden
					lines = append(lines, extra...)
				}
				shelf.lines = append(shelf.lines, lines)
				shelf.fullHeight = max(shelf.fullHeight, len(lines))
			}
		}
		candidate := fitObservationShelves(shelves, budget, m.varCursor)
		heads, cells := 0, 0
		for _, shelf := range candidate {
			heads += len(shelf.indices)
			for _, block := range shelf.lines {
				for _, line := range block[:min(shelf.height, len(block))] {
					cells += line.cells
				}
			}
		}
		if heads > bestRoots || heads == bestRoots && (cells > bestCells || cells == bestCells && hiddenColumns && !horizontal) {
			bestRoots, bestCells = heads, cells
			view, hiddenColumns, offscreen = candidate, horizontal, heads < len(roots)
		}
		if heads == len(roots) && !horizontal && len(candidate) == len(shelves) && allShelvesFit(candidate) {
			break
		}
	}
	label := fmt.Sprintf("数据 · %d 个对象", len(roots))
	if len(m.replay.Current().Globals) > 0 {
		label += " · ⁺ 全局/静态"
	}
	if m.replay.Frame > 0 {
		label += fmt.Sprintf(" · 栈帧 #%d", m.replay.Frame)
	}
	if !m.replay.Current().GlobalsComplete && m.replay.Run.Session.Capabilities.Globals {
		label += " · 部分全局"
	}
	if frame := m.replay.Frame; frame < len(m.replay.Current().Frames) && !m.replay.Current().Frames[frame].VariablesComplete {
		label += " · 部分局部"
	}
	lines := []string{muted.Render(clip(label, w))}
	hidden := hiddenColumns || offscreen
	for _, s := range view {
		hidden = hidden || s.height < s.fullHeight
		for i := 0; i < s.height; i++ {
			var cells []string
			for j, block := range s.lines {
				line := ""
				if i < len(block) {
					line = block[i].text
				}
				cells = append(cells, pad(line, s.widths[j]))
			}
			lines = append(lines, strings.Join(cells, "  "))
		}
	}
	if hidden {
		hint := "部分已采集值未展示 · D 详情"
		if offscreen {
			hint = "↑/↓ 浏览其余对象 · D 详情"
		}
		lines = append(lines, muted.Render(clip(hint, w)))
	}
	return fixed(lines, w, h)
}

func allShelvesFit(shelves []observationShelf) bool {
	for _, s := range shelves {
		if s.height < s.fullHeight {
			return false
		}
	}
	return true
}

func fitObservationShelves(shelves []observationShelf, budget, cursor int) []observationShelf {
	start := 0
	if len(shelves) > budget {
		for i, s := range shelves {
			if cursor >= s.indices[0] && cursor <= s.indices[len(s.indices)-1] {
				start = max(0, min(i-budget+1, len(shelves)-budget))
				break
			}
		}
	}
	view := shelves[start:min(len(shelves), start+budget)]
	remaining := budget - len(view)
	order := make([]int, len(view))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return view[order[i]].fullHeight < view[order[j]].fullHeight })
	for _, i := range order {
		need := view[i].fullHeight - 1
		if need <= 3 && need <= remaining {
			view[i].height += need
			remaining -= need
		}
	}
	for remaining > 0 {
		grew := false
		for i := range view {
			if view[i].height < view[i].fullHeight {
				view[i].height++
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
	return view
}
