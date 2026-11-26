package tui

import (
	"fmt"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
)

// comparisonSummary never treats a missing identity or a partial capture as
// evidence of unchanged data. The scope is the currently viewed frame.
func (m Model) comparisonSummary(w int) string {
	if !m.replay.Step().HasInterval() {
		return muted.Render(clip("观察起点", w))
	}
	rows := m.replay.Rows(nil)
	var labels []string
	unknown, compared, count := false, false, 0
	changedStrings := map[string]bool{}
	for _, row := range rows {
		if row.Raw.Kind == observer.Array || row.Raw.Kind == observer.Struct {
			unknown = unknown || !row.Raw.ChildrenComplete || !row.Reliable || row.Raw.Status != observer.Observed
		} else {
			compared = compared || row.Comparable
			unknown = unknown || !row.Comparable
		}
		if !row.Changed {
			continue
		}
		// Summarize a logical string once, while retaining its changed
		// character children in the detail view and history.
		if changedStrings[row.Parent] {
			continue
		}
		if row.Raw.Kind == observer.String {
			changedStrings[row.Key] = true
		}
		count++
		labels = append(labels, SafeText(row.Path, false)+": "+row.Before+" → "+row.Value)
	}
	for _, s := range []*observer.Snapshot{m.replay.Current(), m.replay.Previous()} {
		if s == nil {
			continue
		}
		if m.replay.Frame >= len(s.Frames) || !s.Frames[m.replay.Frame].VariablesComplete {
			unknown = true
		}
		if m.replay.Run.Session.Capabilities.Globals && !s.GlobalsComplete {
			unknown = true
		}
	}
	if count == 0 {
		switch {
		case !compared:
			return muted.Render(clip("变化无法确认", w))
		case unknown:
			return muted.Render(clip("已比较值无变化 · 部分无法比较", w))
		default:
			return muted.Render(clip("已比较值无变化", w))
		}
	}
	suffix := ""
	if unknown {
		suffix = " · 比较受限"
	}
	for n := min(2, len(labels)); n > 0; n-- {
		label := strings.Join(labels[:n], "；")
		if count > n {
			label += fmt.Sprintf(" +%d项", count-n)
		}
		if cellWidth(label+suffix) <= w {
			return changed.Render(label) + muted.Render(suffix)
		}
	}
	return changed.Render(clip(fmt.Sprintf("%d 项变化", count), w)) + muted.Render(clip(suffix, max(0, w-cellWidth(fmt.Sprintf("%d 项变化", count)))))
}

func panel(title string, lines []string, w, h int) string {
	if w < 4 || h < 2 {
		return fixed(lines, w, h)
	}
	label := clip(" "+title+" ", w-2)
	border := func(s string) string { return muted.Render(s) }
	out := []string{border("╭") + accent.Render(label) + border(strings.Repeat("─", max(0, w-2-cellWidth(label)))+"╮")}
	body := strings.Split(fixed(lines, w-2, h-2), "\n")
	if h > 2 {
		for _, line := range body {
			out = append(out, border("│")+line+border("│"))
		}
	}
	out = append(out, border("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(out, "\n")
}

func (m Model) outputPreview(w, h int) []string {
	out, errOut := m.replay.Output()
	before := 0
	if p := m.replay.Previous(); p != nil {
		before = p.OutputOffsets.StdoutBytes
	}
	// The first pause is a baseline, not an observed transition.
	if !m.replay.Step().HasInterval() {
		before = len(out)
	}
	before = max(0, min(before, len(out)))
	label := "输出"
	if len(out) > before {
		label += fmt.Sprintf(" +%dB", len(out)-before)
	}
	if len(errOut) > 0 {
		label += " · stderr"
	}
	if m.replay.Run.Output.StdoutTruncated {
		label += " · 截断"
	}
	lines := []string{accent.Render(clip(label, w))}
	if len(out) == 0 {
		return append(lines, muted.Render(clip("尚无已捕获输出", w)))
	}
	// Tail preview, retaining spaces and marking only bytes actually added
	// between snapshots. A UTF-8 rune crossing an offset is marked as a whole.
	text := string(out)
	parts := strings.Split(text, "\n")
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	start := max(0, len(parts)-max(1, h-1))
	offset := 0
	for i, line := range parts {
		if i >= start {
			// Hide CR only when it belongs to a CRLF line ending.
			display := line
			if offset+len(line) < len(text) {
				display = strings.TrimSuffix(display, "\r")
			}
			var b strings.Builder
			width := 0
			for j, r := range display {
				piece := SafeText(string(r), false)
				n := len(string(r))
				if width+cellWidth(piece) > w {
					break
				}
				if offset+j+n > before {
					b.WriteString(changed.Render(piece))
				} else {
					b.WriteString(piece)
				}
				width += cellWidth(piece)
			}
			lines = append(lines, b.String())
		}
		offset += len(line) + 1
	}
	return lines
}

func (m Model) inputPreview(w, h int) []string {
	lines := []string{accent.Render(clip("输入 · 样例", w))}
	if len(m.config.Stdin) == 0 {
		return append(lines, muted.Render("（空）"))
	}
	parts := strings.Split(SafeText(strings.ReplaceAll(string(m.config.Stdin), "\r\n", "\n"), true), "\n")
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	limit := max(1, h-1)
	for i, line := range parts[:min(limit, len(parts))] {
		if i == limit-1 && len(parts) > limit {
			line = clip(line, max(0, w-2)) + " …"
		}
		lines = append(lines, clip(line, w))
	}
	return lines
}

func (m Model) observationWorkspace(w, h int) string {
	topH := 7
	if h >= 32 {
		topH = 9
	}
	if w < 76 || h < 18 {
		topH = 5
		if h >= 18 {
			topH = 6
		}
	}
	topH = min(topH, h-4)
	var top string
	if w >= 76 && topH >= 7 {
		left := w*3/5
		right := w-left
		source := panel("执行 · 区间", m.executionContext(left-2), left, topH)
		inner := topH-2
		inH := inner/2
		io := strings.Split(fixed(m.inputPreview(right-2, inH), right-2, inH), "\n")
		io = append(io, m.outputPreview(right-2, inner-inH)...)
		output := panel("输入输出", io, right, topH)
		a, b := strings.Split(source, "\n"), strings.Split(output, "\n")
		var joined []string
		for i := range a {
			joined = append(joined, a[i]+b[i])
		}
		top = strings.Join(joined, "\n")
	} else {
		// Narrow terminals retain both streams without taking a full second pane.
		context := m.executionContext(w-2)
		lines := []string{context[0]}
		if len(context) > 2 && topH >= 6 {
			lines = append(lines, context[1])
		}
		lines = append(lines, context[len(context)-1])
		in, out := m.inputPreview(w-2, 2), m.outputPreview(w-2, 2)
		if topH >= 6 {
			lines = append(lines, clip("输入: "+strings.Join(strings.Split(SafeText(string(m.config.Stdin), true), "\n"), " "), w-2))
			_ = in
			lines = append(lines, clip("输出: "+SafeText(string(m.configuredOutput()), false), w-2))
		} else {
			lines = append(lines, out[0]+muted.Render(" · I 输入输出"))
		}
		top = panel("执行 / 输入输出", lines, w, topH)
	}
	dataH := h-topH
	data := strings.Split(m.dataFirstPanel(w-2, dataH-2), "\n")
	return top+"\n"+panel("数据", data, w, dataH)
}

func (m Model) configuredOutput() []byte {
	out, _ := m.replay.Output()
	if len(out) == 0 {
		return []byte("尚无已捕获输出")
	}
	return out
}
