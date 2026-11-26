package tui

import (
	"fmt"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
)

// Summary reports visible differences, not proven assignment events.
func (m Model) comparisonSummary(w int) string {
	if !m.replay.Step().HasInterval() {
		return ""
	}
	var labels []string
	changedStrings := map[string]bool{}
	for _, row := range m.replay.Rows(nil) {
		if !row.Changed || changedStrings[row.Parent] {
			continue
		}
		if row.Raw.Kind == observer.String {
			changedStrings[row.Key] = true
		}
		labels = append(labels, SafeText(row.Path, false)+": "+row.Before+" → "+row.Value)
	}
	if len(labels) == 0 {
		return muted.Render(clip("无可见变化", w))
	}
	for n := min(2, len(labels)); n > 0; n-- {
		label := strings.Join(labels[:n], "；")
		if len(labels) > n {
			label += fmt.Sprintf(" +%d项", len(labels)-n)
		}
		if cellWidth(label) <= w {
			return changed.Render(label)
		}
	}
	return changed.Render(clip(fmt.Sprintf("%d 项变化", len(labels)), w))
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
	out := m.replay.Run.Output.Stdout
	label := "最终输出"
	if m.replay.Run.Reason != observer.Exited || m.replay.Run.Output.StdoutTruncated {
		label = "已记录输出"
	}
	lines := []string{accent.Render(clip(label, w))}
	if len(out) == 0 {
		return append(lines, muted.Render(clip("（空）", w)))
	}
	parts := strings.Split(SafeText(strings.ReplaceAll(string(out), "\r\n", "\n"), true), "\n")
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	limit := max(1, h-1)
	for i, line := range parts[:min(limit, len(parts))] {
		if cellWidth(line) > w || i == limit-1 && len(parts) > limit {
			line = clip(line, max(0, w-2)) + " …"
		}
		lines = append(lines, clip(line, w))
	}
	return lines
}

func (m Model) inputPreview(w, h int) []string {
	lines := []string{accent.Render(clip("输入", w))}
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
		topH = 6
		if h >= 18 {
			topH = 7
		}
	}
	topH = min(topH, h-4)
	var top string
	if w >= 76 && topH >= 7 {
		left := w * 3 / 5
		right := w - left
		source := panel("代码", m.executionContext(left-2), left, topH)
		inner := topH - 2
		inH := inner / 2
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
		context := m.executionContext(w - 2)
		current := 1
		if m.height >= 24 {
			current = 2
		}
		var lines []string
		if topH >= 7 {
			lines = append(lines, context[0])
		}
		lines = append(lines, context[current], context[len(context)-1])
		in, out := m.inputPreview(w-8, 2), m.outputPreview(w-8, 2)
		lines = append(lines, accent.Render("输入 ")+in[1], out[0]+" "+out[1])

		top = panel("代码 / 输入输出", lines, w, topH)
	}
	dataH := h - topH
	data := strings.Split(m.dataFirstPanel(w-2, dataH-2), "\n")
	return top + "\n" + panel("数据", data, w, dataH)
}
