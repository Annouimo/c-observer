package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/Routilet/c-observer/internal/observer"
)

func (m *Model) openInspector(mode string) {
	target, ok := m.selectedVariable()
	if !ok {
		return
	}
	m.detailTree = false
	m.notice = ""
	if mode == "history" && !target.Reliable {
		m.notice = "对象身份无法确认，不能生成可靠的跨快照历史。"
		return
	}
	if mode == "range" && target.Raw.Kind != observer.Array {
		m.notice = "请选择一个数组，再调整采集范围。"
		return
	}
	m.mode, m.target = mode, target
	m.modalScroll, m.modalCol = 0, 0
	if mode == "history" {
		m.history = m.replay.History(target)
	}
	if mode == "range" {
		m.openArrayRange(target)
	}
}

func (m *Model) openArrayRange(row VariableRow) {
	m.target, m.mode, m.rangeField = row, "range", 0
	start := 0
	if len(row.Raw.Children) > 0 && row.Raw.Children[0].Index != nil {
		start = *row.Raw.Children[0].Index
	}
	count := m.config.Limits.MaxArrayElements
	if count <= 0 {
		count = 16
	}
	m.rangeInput = [2]string{fmt.Sprint(start), fmt.Sprint(count)}
}

func (m *Model) handleInspector(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()
	if k == "esc" {
		if m.mode == "value" {
			m.mode = "detail"
			return nil
		}
		m.mode = ""
		m.notice = ""
		return nil
	}
	if k == "ctrl+c" || k == "q" {
		return tea.Quit
	}
	switch m.mode {
	case "value":
		switch k {
		case "up", "pgup":
			step := 1
			if k == "pgup" {
				step = max(1, m.height-4)
			}
			m.valueScroll = max(0, m.valueScroll-step)
		case "down", "pgdown":
			step := 1
			if k == "pgdown" {
				step = max(1, m.height-4)
			}
			m.valueScroll = min(m.valueScroll+step, max(0, len(m.valueLines(m.width))-max(1, m.height-3)))
		}
	case "detail":
		switch k {
		case "left", "right", "home", "end":
			switch k {
			case "left":
				m.navigate(-1)
			case "right":
				m.navigate(1)
			case "home":
				m.seek(0)
			case "end":
				m.seek(len(m.replay.Run.Snapshots))
			}
			m.modalScroll = 0
		case "up":
			m.modalScroll = max(0, m.modalScroll-1)
		case "down":
			if row, ok := m.inspectedRow(); ok {
				m.modalScroll = min(m.modalScroll+1, max(0, len(m.detailTable(row).Rows)-1))
			}
		case "[":
			m.modalCol = max(0, m.modalCol-1)
		case "]":
			if row, ok := m.inspectedRow(); ok {
				m.modalCol = min(m.modalCol+1, max(0, len(m.detailTable(row).Headers)-2))
			}
		case "t":
			m.detailTree = !m.detailTree
			m.modalScroll, m.modalCol = 0, 0
		case "enter":
			if row, ok := m.inspectedRow(); ok {
				t := m.detailTable(row)
				if m.modalScroll < len(t.Rows) && len(t.Rows[m.modalScroll]) > 0 {
					cells := t.Rows[m.modalScroll]
					m.detailValue = cells[min(len(cells)-1, m.modalCol+1)]
					m.valueScroll = 0
					m.mode = "value"
				}
			}
		case "v":
			if row, ok := m.inspectedRow(); ok {
				m.toggleFull(row)
				m.modalScroll = 0
				m.modalCol = 0
			}
		case "h":
			if row, ok := m.inspectedRow(); ok && row.Reliable {
				m.target = row
				m.mode = "history"
				m.history = m.replay.History(row)
				m.modalScroll = 0
			}
		case "a":
			if row, ok := m.inspectedRow(); ok {
				if m.detailTree {
					table := m.detailTable(row)
					if m.modalScroll < len(table.Rows) {
						target := table.Rows[m.modalScroll][1].Key
						row, ok = findRow(m.replay.Rows(nil), target)
					}
				}
				if ok && row.Raw.Kind == observer.Array {
					m.openArrayRange(row)
				}
			}
		}
	case "history", "line-history":
		switch k {
		case "up":
			m.modalScroll = max(0, m.modalScroll-1)
		case "down":
			m.modalScroll = min(m.modalScroll+1, max(0, len(m.history)-1))
		case "home":
			m.modalScroll = 0
		case "end":
			m.modalScroll = max(0, len(m.history)-1)
		case "enter":
			if len(m.history) > 0 {
				m.seek(m.history[m.modalScroll].Position)
				if m.mode == "line-history" {
					m.focus = 0
				} else {
					m.focus = 1
					m.selectHistoryTarget()
				}
				m.mode = ""
			}
		}
	case "range":
		switch k {
		case "tab", "shift+tab", "up", "down":
			m.rangeField = 1 - m.rangeField
		case "backspace":
			s := m.rangeInput[m.rangeField]
			if len(s) > 0 {
				m.rangeInput[m.rangeField] = s[:len(s)-1]
			}
		case "ctrl+u":
			m.rangeInput[m.rangeField] = ""
		case "enter":
			decl := m.target.Raw.Declaration
			for _, row := range m.replay.Rows(nil) {
				if row.Depth == 0 && (row.Key == m.target.Key || strings.HasPrefix(m.target.Key, row.Key+"/")) {
					decl = row.Raw.Declaration
					break
				}
			}
			r, err := rangeFor(m.target, decl, m.rangeInput[0], m.rangeInput[1], m.config.Limits.MaxArrayElements)
			if err != nil {
				m.notice = err.Error()
				return nil
			}
			if m.demo {
				m.notice = "合成预览不会重新采集；真实引擎接入后此操作将重新运行。"
				return nil
			}
			if m.engine == nil {
				m.notice = "尚未连接观察引擎。"
				return nil
			}
			next := make([]observer.ArrayRange, 0, len(m.config.ArrayRanges)+1)
			for _, old := range m.config.ArrayRanges {
				if old.Path != r.Path || !sameDeclaration(old.Declaration, r.Declaration) {
					next = append(next, old)
				}
			}
			m.config.ArrayRanges = append(next, r)
			m.mode = ""
			return m.start()
		default:
			if len(m.rangeInput[m.rangeField])+len(k) <= 18 {
				digits := len(k) > 0
				for _, r := range k {
					if r < '0' || r > '9' {
						digits = false
						break
					}
				}
				if digits {
					m.rangeInput[m.rangeField] += k
				}
			}
		}
	}
	return nil
}

func sameDeclaration(a, b *observer.SourceLocation) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (m Model) inspectedRow() (VariableRow, bool) {
	if !m.target.Reliable {
		// An unstable object is inspectable only at the originally selected pause.
		if m.replay.Position != m.targetPosition {
			return VariableRow{}, false
		}
	}
	row, ok := findRow(m.replay.Rows(nil), m.target.Key)
	if m.target.Reliable && ok && !row.Reliable {
		return VariableRow{}, false
	}
	return row, ok
}

func (m Model) detailTable(row VariableRow) DataTable {
	all := m.replay.Rows(nil)
	if !m.detailTree {
		return m.dataTable(row, all)
	}
	table := DataTable{Headers: []string{"对象", "值", "类型"}}
	for _, child := range all {
		if child.Key == row.Key || strings.HasPrefix(child.Key, row.Key+"/") {
			table.Rows = append(table.Rows, []Cell{{Text: child.Path}, cellFor(child), {Text: child.Type}})
		}
	}
	return table
}

func (m Model) valueLines(width int) []string {
	c := m.detailValue
	value := c.Raw
	if !c.Observed {
		value = c.Text
	}
	if c.Truncated {
		value += "\n[采集时已截断]"
	}
	if value == "" {
		value = "（无显示值）"
	}
	return strings.Split(runewidth.Wrap(value, max(1, width)), "\n")
}

// renderTable repeats the index column while allowing horizontal column paging.
// Display widths are measured in terminal cells, not bytes.
func renderTable(t DataTable, w, h, offset, colOffset int) []string {
	if len(t.Headers) == 0 || h <= 0 {
		return nil
	}
	if w < 12 {
		return []string{"窗口过窄"}
	}
	cols := []int{0}
	widths := []int{min(max(8, runewidth.StringWidth(t.Headers[0])), max(8, w/4))}
	for _, r := range t.Rows {
		if len(r) > 0 {
			widths[0] = min(max(widths[0], runewidth.StringWidth(r[0].Text)), max(8, w/4))
		}
	}
	used := widths[0]
	start := min(max(1, 1+colOffset), max(1, len(t.Headers)-1))
	for i := start; i < len(t.Headers); i++ {
		desired := max(8, runewidth.StringWidth(t.Headers[i]))
		for _, r := range t.Rows {
			if i < len(r) {
				size := runewidth.StringWidth(r[i].Text)
				if r[i].Changed {
					size += 2
				}
				desired = max(desired, size)
			}
		}
		cw := min(desired, 28)
		avail := w - used - 3
		if avail < 6 {
			break
		}
		if len(cols) > 1 && cw > avail {
			break
		}
		cw = min(cw, avail)
		cols = append(cols, i)
		widths = append(widths, cw)
		used += cw + 3
	}
	draw := func(cells []Cell, header, active bool) string {
		var parts []string
		for j, col := range cols {
			cell := Cell{Text: "未记录"}
			if col < len(cells) {
				cell = cells[col]
			}
			raw := cell.Text
			if cell.Changed {
				raw = "* " + raw
			}
			text := pad(clip(raw, widths[j]), widths[j])
			if header {
				text = muted.Render(text)
			} else if cell.Changed {
				text = changed.Render(text)
			}
			if active && (col == start || len(cols) == 1) {
				text = selected.Render(text)
			}
			parts = append(parts, text)
		}
		return strings.Join(parts, " │ ")
	}
	headers := make([]Cell, len(t.Headers))
	for i, x := range t.Headers {
		headers[i] = Cell{Text: x}
	}
	lines := []string{draw(headers, true, false)}
	if h > 1 {
		lines = append(lines, muted.Render(strings.Repeat("─", min(w, used))))
	}
	startRow := max(0, min(offset-max(1, h-2)+1, max(0, len(t.Rows)-max(1, h-2))))
	for i := startRow; i < min(len(t.Rows), startRow+max(0, h-2)); i++ {
		lines = append(lines, draw(t.Rows[i], false, i == offset))
	}
	if len(t.Rows) == 0 && len(lines) < h {
		lines = append(lines, "无已记录子元素")
	}
	return lines
}

func (m Model) inspector(w, h int) string {
	lines := []string{accent.Render(clip("C Observer / "+SafeText(m.target.Path, false), w))}
	switch m.mode {
	case "value":
		label := m.detailValue.Path
		if label == "" {
			label = "选中单元格"
		}
		lines = append(lines, muted.Render(clip("采集原值 · "+label, w)))
		values := m.valueLines(w)
		start := min(m.valueScroll, max(0, len(values)-max(1, h-3)))
		lines = append(lines, values[start:min(len(values), start+max(1, h-3))]...)
		lines = append(lines, muted.Render("↑/↓ 滚动  Esc 返回详情"))
	case "detail":
		row, ok := m.inspectedRow()
		lines = append(lines, muted.Render(clip(fmt.Sprintf("数据详情 · 快照 %d / %d", m.replay.Position+1, len(m.replay.Run.Snapshots)), w)))
		if !ok {
			lines = append(lines, "当前快照中无法确认此对象；不显示旧值。")
		} else {
			lines = append(lines, clip(row.Type+" · "+cellFor(row).Text, w))
			note := m.rangeLabel(row)
			if !row.Reliable || row.Changed && !row.ReliableChange {
				if note != "" {
					note += " · "
				}
				note += "名称对应，仅供观察"
			}
			lines = append(lines, muted.Render(clip(note, w)))
			t := m.detailTable(row)
			label := "关注范围"
			if m.detailTree {
				label = "全部已采集成员"
			}
			lines = append(lines, muted.Render(clip(fmt.Sprintf("%s · %d 行", label, len(t.Rows)), w)))
			lines = append(lines, renderTable(t, w, max(1, h-8), min(m.modalScroll, max(0, len(t.Rows)-1)), m.modalCol)...)
		}
		lines = append(lines, muted.Render(clip("←/→ 回放  ↑/↓ 选行  [/] 列  Enter 原值  T 全部成员  V 范围  H 历史  A 采集  Esc 返回", w)))
	case "history", "line-history":
		label := "变量历史 · 仅可靠身份与已记录变化，非完整赋值日志"
		if m.mode == "line-history" {
			label = fmt.Sprintf("本行历史 · %d 次记录暂停（非准确执行次数）", len(m.history))
		}
		lines = append(lines, muted.Render(clip(label, w)))
		if len(m.history) == 0 {
			lines = append(lines, "无可确认的变化记录。")
		}
		n := max(1, h-5)
		start := max(0, min(m.modalScroll-n+1, max(0, len(m.history)-n)))
		for i := start; i < min(len(m.history), start+n); i++ {
			e := m.history[i]
			mark := " "
			if i == m.modalScroll {
				mark = ">"
			}
			line := clip(fmt.Sprintf("%s #%d  %s:%d  %s", mark, e.Position+1, SafeText(e.Location.File, false), e.Location.Line, e.Text), w)
			if e.Changed {
				line = changed.Render(line)
			}
			if i == m.modalScroll {
				line = selected.Render(line)
			}
			lines = append(lines, line)
		}
		lines = append(lines, muted.Render(clip("↑/↓ 选择  Enter 跳转到快照  Home/End 首尾  Esc 返回", w)))
	case "range":
		lines = append(lines, "调整数组范围（需要重新运行）", "")
		for i, label := range []string{"起始下标", "元素数量"} {
			mark := " "
			if m.rangeField == i {
				mark = ">"
			}
			lines = append(lines, clip(mark+" "+label+"  "+m.rangeInput[i]+"_", w))
		}
		lines = append(lines, "", clip("仅替换下一次运行的采集页，不补取旧历史。", w))
		if m.notice != "" {
			lines = append(lines, warning.Render(clip(m.notice, w)))
		}
		lines = append(lines, muted.Render(clip("Tab 切换  Ctrl+U 清空  Enter 重新运行  Esc 返回", w)))
	}
	return fixed(lines, w, h)
}
