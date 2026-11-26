package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/Routilet/c-observer/internal/observer"
)

type Model struct {
	replay                                        Replay
	config                                        observer.LaunchConfig
	engine                                        observer.Engine
	demo                                          bool
	offline                                       bool
	record                                        *ObservationRecord
	observerVersion, exportPath                   string
	width, height                                 int
	focus, sourceScroll, varCursor, consoleScroll int
	consoleTab                                    int
	running, stopping, quitting                   bool
	cancel                                        context.CancelFunc
	started                                       time.Time
	err                                           string
	mode, notice                                  string
	standalone, settingUp                         bool
	setup                                         SetupState
	target                                        VariableRow
	targetPosition, modalScroll, modalCol         int
	detailTree                                    bool
	detailValue                                   Cell
	valueScroll                                   int
	history                                       []HistoryEntry
	rangeInput                                    [2]string
	rangeField                                    int
	displayInput                                  [4]string
	displayFields                                 int
	fullArrays                                    map[string]bool
	manualRanges                                  map[string]FocusRange
	navigation                                    []int
	navLabel                                      string
	sourceLine, sourceColumn                      int
	menuCursor                                    int
	jumpInput                                     string
}
type runDone struct{ result observer.RunResult }
type clockTick time.Time

func NewPreview(f Fixture) Model {
	m := Model{replay: NewReplay(f.Run), demo: true, width: 100, height: 30, focus: 1, consoleTab: 1}
	m.config.Stdin = []byte(f.Stdin)
	if f.ExpectedStdout != nil {
		b := []byte(*f.ExpectedStdout)
		m.config.ExpectedStdout = &b
	}
	return m
}

// NewLive is the integration boundary; the engine is injected by the caller.
func NewLive(engine observer.Engine, config observer.LaunchConfig) Model {
	return Model{engine: engine, config: config, width: 100, height: 30, focus: 1, consoleTab: 1}
}

func (m Model) Init() tea.Cmd {
	if m.demo || m.engine == nil || m.settingUp {
		return nil
	}
	return func() tea.Msg { return startRun{} }
}

type startRun struct{}

func tick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return clockTick(t) })
}
func (m *Model) start() tea.Cmd {
	if m.engine == nil || m.running {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.started = time.Now()
	m.running = true
	m.stopping = false
	m.err = ""
	m.record = nil
	engine, cfg := m.engine, m.config
	return tea.Batch(tick(), func() tea.Msg { return runDone{engine.Run(ctx, cfg)} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case startRun:
		cmd := m.start()
		return m, cmd
	case clockTick:
		if m.running {
			return m, tick()
		}
	case runDone:
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.running = false
		m.stopping = false
		if m.quitting {
			return m, tea.Quit
		}
		if err := ValidateRun(msg.result); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.replay = NewReplay(msg.result)
		record := captureRecord(m.observerVersion, m.config, msg.result)
		m.record = &record
		m.fullArrays = nil
		m.manualRanges = nil
		m.navigation = nil
		m.navLabel = ""
		m.sourceLine = 0
		m.sourceColumn = 0
		m.sourceScroll, m.varCursor, m.consoleScroll = 0, 0, 0
		m.mode, m.notice = "", ""
		m.focus = 1
		if len(msg.result.Snapshots) == 0 {
			m.consoleTab = 1
		}
	case tea.KeyMsg:
		if m.settingUp {
			cmd := m.setupKey(msg)
			return m, cmd
		}
		k := msg.String()
		if m.running {
			if k == "esc" || k == "ctrl+c" || k == "q" {
				m.stopping = true
				if k == "ctrl+c" || k == "q" {
					m.quitting = true
				}
				if m.cancel != nil {
					m.cancel()
				}
			}
			return m, nil
		}
		if m.mode != "" {
			if m.workspaceMode() {
				cmd := m.handleWorkspace(msg)
				return m, cmd
			}
			cmd := m.handleInspector(msg)
			return m, cmd
		}
		m.notice = ""
		switch k {
		case "c":
			m.focus = 0
		case "i":
			m.focus = 2
		case "f":
			m.mode = "filter"
			m.menuCursor = 0
		case "s":
			m.mode = "stack"
			m.menuCursor = m.replay.Frame
		case "j":
			m.mode = "jump"
			m.jumpInput = ""
		case "?":
			m.mode = "help"
		case "w":
			if m.record != nil && !m.offline && !m.demo {
				m.exportPath = ""
				m.mode = "record-confirm"
			}
		case "o":
			if m.focus == 1 && !m.replay.End() {
				m.mode = "object"
				m.menuCursor = 0
			}
		case "v":
			if row, ok := m.selectedVariable(); ok {
				m.toggleFull(row)
			}
		case "[":
			if m.focus == 0 {
				m.sourceColumn = max(0, m.sourceColumn-8)
			}
		case "]":
			if m.focus == 0 {
				m.sourceColumn += 8
			}
		case "d", "h", "a":
			if k == "h" && m.focus == 0 {
				if loc := m.selectedLocation(); loc.File != "" && loc.Line > 0 {
					m.mode = "line-history"
					m.target = VariableRow{Path: fmt.Sprintf("%s:%d", loc.File, loc.Line)}
					m.history = m.replay.LineHistory(loc)
					m.modalScroll = 0
				}
				break
			}
			if m.focus == 1 && !m.replay.End() {
				mode := map[string]string{"d": "detail", "h": "history", "a": "range"}[k]
				m.openInspector(mode)
				m.targetPosition = m.replay.Position
			}
		case "esc":
			if m.focus != 1 {
				m.focus = 1
				break
			}
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		case "e":
			if m.standalone {
				m.setup = setupFrom(m.config)
				m.settingUp = true
			}
		case "r":
			if m.demo || m.offline {
				m.replay.Seek(0)
				m.resetScroll()
				break
			}
			cmd := m.start()
			return m, cmd
		case "left":
			m.navigate(-1)
		case "right":
			m.navigate(1)
		case "home":
			m.seek(0)
		case "end":
			m.seek(len(m.replay.Run.Snapshots))
		case "tab":
			m.focus = (m.focus + 1) % 3
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
		case "up", "down", "pgup", "pgdown":
			d := 1
			if k == "up" || k == "pgup" {
				d = -1
			}
			if k == "pgup" || k == "pgdown" {
				d *= max(1, m.height/3)
			}
			switch m.focus {
			case 0:
				m.moveSource(d)
			case 1:
				if m.replay.End() {
					m.moveSource(d)
					break
				}
				m.varCursor = max(0, min(m.varCursor+d, len(rootRows(m.replay.Rows(nil)))-1))
			case 2:
				length := len(m.consoleLines(m.consoleTab, m.consoleWidth()))
				if m.consoleTab == 4 {
					length = m.comparisonLength(m.consoleWidth())
				}
				m.consoleScroll = max(0, min(m.consoleScroll+d, max(0, length-1)))
			}
		case "enter":
			if m.focus == 1 {
				m.openInspector("detail")
				m.targetPosition = m.replay.Position
			}
			if m.focus == 0 && m.replay.Current() != nil {
				m.mode = "stack"
				m.menuCursor = m.replay.Frame
			}
			if m.focus == 2 {
				m.consoleTab = (m.consoleTab + 1) % 5
				m.consoleScroll = 0
			}
		}
	}
	return m, nil
}
func (m *Model) clampSourceScroll() {
	if m.replay.End() {
		m.sourceScroll = max(0, min(m.sourceScroll, len(m.replay.Run.Diagnostics)+5))
		return
	}

}
func (m *Model) seek(pos int) {
	rows := rootRows(m.replay.Rows(nil))
	var key string
	if m.varCursor >= 0 && m.varCursor < len(rows) && rows[m.varCursor].Reliable {
		key = rows[m.varCursor].Key
	}
	m.replay.Seek(pos)
	m.sourceLine = 0
	m.sourceColumn = 0
	m.resetScroll()
	if key != "" {
		for i, row := range rootRows(m.replay.Rows(nil)) {
			if row.Key == key {
				m.varCursor = i
				break
			}
		}
	}
	if m.replay.End() {
		m.consoleTab = 1
		if m.config.ExpectedStdout != nil {
			m.consoleTab = 4
		}
	}
}
func (m *Model) resetScroll() {
	m.sourceScroll, m.consoleScroll = 0, 0
	rows := rootRows(m.replay.Rows(nil))
	m.varCursor = max(0, min(m.varCursor, len(rows)-1))
}

func reasonLabel(reason observer.FinishReason) string {
	switch reason {
	case observer.Exited:
		return "正常退出"
	case observer.NonzeroExit:
		return "非零退出"
	case observer.Signaled:
		return "异常终止"
	case observer.TimedOut:
		return "运行超时"
	case observer.RequestTimedOut:
		return "调试请求超时"
	case observer.InvalidConfig:
		return "配置错误"
	case observer.Cancelled:
		return "已取消"
	case observer.LimitReached:
		return "达到采集上限"
	case observer.CompileFailed:
		return "编译失败"
	case observer.BackendFailed:
		return "调试器错误"
	default:
		return SafeText(string(reason), false)
	}
}

var (
	muted    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "245"})
	accent   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "25", Dark: "75"}).Bold(true)
	changed  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "114"})
	warning  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "221"})
	failure  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "124", Dark: "203"})
	selected = lipgloss.NewStyle().Bold(true).Underline(true)
)

func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= w {
		return s
	}
	return runewidth.Truncate(s, w, "…")
}
func pad(s string, w int) string { return s + strings.Repeat(" ", max(0, w-lipgloss.Width(s))) }
func fixed(lines []string, w, h int) string {
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = pad(lines[i], w)
	}
	return strings.Join(lines, "\n")
}
func head(s string, focused bool, w int) string {
	s = clip(s, w)
	if focused {
		return accent.Render(s)
	}
	return muted.Render(s)
}
func sourceFor(run observer.RunResult, path string) string {
	for _, s := range run.Session.Sources {
		if s.Path == path {
			return s.Text
		}
	}
	return ""
}
func (m Model) source(w, h int) string {
	s := m.replay.Current()
	sourceTitle := " 源代码 · 暂停位置"
	if m.replay.Frame > 0 {
		sourceTitle = " 源代码 · 所选调用帧"
	}
	lines := []string{head(sourceTitle, m.focus == 0, w)}
	if s == nil {
		return fixed(append(lines, muted.Render("没有当前源码位置。")), w, h)
	}
	loc := m.replay.ViewedLocation()
	lines = append(lines, muted.Render(clip(SafeText(fmt.Sprintf("%s:%d  %s", loc.File, loc.Line, loc.Function), false), w)))
	src := sourceFor(m.replay.Run, loc.File)
	if loc.File == "" || src == "" || loc.Line <= 0 {
		return fixed(append(lines, clip("未映射: "+SafeText(loc.ReportedFile, false), w)), w, h)
	}
	all := strings.Split(SafeText(src, true), "\n")
	n := max(1, h-2)
	center := loc.Line
	if m.sourceLine > 0 {
		center = m.sourceLine
	}
	start := max(0, min(center-1-n/2+m.sourceScroll, max(0, len(all)-n)))
	// Lex the whole file so multiline comments remain highlighted when scrolled.
	tokens := lexC(strings.Join(all, "\n"))
	for i := start; i < min(len(all), start+n); i++ {
		mark := " "
		if i+1 == loc.Line {
			mark = ">"
		} else if m.sourceLine == i+1 {
			mark = ":"
		}
		prefix := fmt.Sprintf("%s%4d ", mark, i+1)
		if i+1 == loc.Line {
			prefix = accent.Render(prefix)
		} else if m.sourceLine == i+1 {
			prefix = selected.Render(prefix)
		} else {
			prefix = muted.Render(prefix)
		}
		lines = append(lines, prefix+renderTokens(skipTokenCells(tokens[i], m.sourceColumn), max(0, w-runewidth.StringWidth(fmt.Sprintf("%s%4d ", mark, i+1)))))
	}
	return fixed(lines, w, h)
}
func (m Model) consoleText() string { return m.consoleTextFor(m.consoleTab) }
func (m Model) consoleTextFor(tab int) string {
	out, errOut := m.replay.Output()
	var data string
	switch tab {
	case 0:
		data = string(m.config.Stdin)
	case 1:
		data = string(out)
	case 2:
		data = string(errOut)
	case 3:
		if m.config.ExpectedStdout != nil {
			data = string(*m.config.ExpectedStdout)
		} else {
			data = "（未提供）"
		}
	}
	if data == "" {
		data = "（空）"
	}
	if tab == 1 && m.replay.Run.Output.StdoutTruncated || tab == 2 && m.replay.Run.Output.StderrTruncated {
		data += "\n[运行输出已截断]"
	}
	return SafeText(data, true)
}
func (m Model) consoleWidth() int { return m.width }
func (m Model) consoleLines(tab, w int) []string {
	return strings.Split(runewidth.Wrap(m.consoleTextFor(tab), max(1, w)), "\n")
}
func (m Model) console(w, h int) string {
	return m.consoleView(m.consoleTab, w, h, m.consoleScroll, m.focus == 2)
}
func (m Model) consoleView(tab, w, h, scroll int, focused bool) string {
	if tab == 4 {
		m.consoleScroll = scroll
		return m.outputComparison(w, h)
	}
	labels := []string{"输入", "实际输出", "错误流", "预期输出"}
	lines := []string{head(" "+labels[tab], focused, w)}
	a := m.consoleLines(tab, w)
	start := max(0, min(scroll, max(0, len(a)-1)))
	for _, l := range a[start:min(len(a), start+max(0, h-1))] {
		lines = append(lines, clip(l, w))
	}
	return fixed(lines, w, h)
}
func (m Model) ending(w, h int) string {
	r := m.replay.Run
	status := fmt.Sprintf("END · %s", reasonLabel(r.Reason))
	if r.ExitCode != nil {
		status += fmt.Sprintf(" · 退出码 %d", *r.ExitCode)
	}
	st := warning
	if r.Reason == observer.Exited {
		st = accent
	}
	if r.Reason == observer.CompileFailed || r.Reason == observer.BackendFailed || r.Reason == observer.Signaled {
		st = failure
	}
	lines := []string{st.Render(clip(status, w)), muted.Render(clip(fmt.Sprintf("已记录 %d 次暂停；无活动源码位置或变量。", len(r.Snapshots)), w))}
	if r.Message != "" {
		lines = append(lines, clip(SafeText(r.Message, false), w))
	}
	if r.CleanupError != "" {
		lines = append(lines, failure.Render(clip("清理失败: "+SafeText(r.CleanupError, false), w)))
	}
	if m.config.ExpectedStdout != nil && r.Reason == observer.Exited && !r.Output.StdoutTruncated {
		match := "输出有差异 · 逐字节比较"
		if string(r.Output.Stdout) == string(*m.config.ExpectedStdout) {
			match = "输出一致 · 逐字节比较"
		}
		lines = append(lines, clip(match, w))
		if equal, applied := sampleEqual(*m.config.ExpectedStdout, r.Output.Stdout, m.config.Comparison); applied {
			label := "按样例规则：不一致"
			if equal {
				label = "按样例规则：一致"
			}
			lines = append(lines, clip(label, w))
		}
	}
	if len(r.Output.Stdout) > 0 {
		first, _, _ := strings.Cut(SafeText(string(r.Output.Stdout), true), "\n")
		lines = append(lines, clip("标准输出  "+first, w))
	}
	lines = append(lines, muted.Render(clip("I 查看完整输入输出", w)))
	for _, d := range r.Diagnostics {
		at := ""
		if d.Location != nil {
			at = fmt.Sprintf("%s:%d ", d.Location.File, d.Location.Line)
		}
		lines = append(lines, clip(SafeText(at+d.Severity+": "+d.Message, false), w))
	}
	start := max(0, min(m.sourceScroll, max(0, len(lines)-h)))
	return fixed(lines[start:], w, h)
}
func (m Model) View() string {
	w, h := m.width, m.height
	if w < 45 || h < 14 {
		return "请将终端调整至至少 45 × 14。Q 退出。"
	}
	if m.settingUp {
		return m.setupView(w, h)
	}
	title := "C Observer"
	if m.demo {
		title += " · 合成数据预览"
	} else if m.offline {
		title += " · 离线记录"
	}
	if m.running {
		status := "正在编译并采集运行状态…"
		if m.stopping {
			status = "正在停止，等待调试器清理…"
		}
		return accent.Render(title) + "\n\n" + status + "\n" + time.Since(m.started).Round(100*time.Millisecond).String() + "\n\nEsc 取消 · Q 取消并退出"
	}
	if m.err != "" {
		return failure.Render("运行记录无效: "+SafeText(m.err, false)) + "\nR 重试 · Q 退出"
	}
	if m.mode != "" {
		if m.workspaceMode() {
			return m.workspaceOverlay(w, h)
		}
		return m.inspector(w, h)
	}
	mainH := h - 4
	var body string
	switch {
	case m.focus == 2:
		body = m.console(w, mainH)
	case m.replay.End():
		body = m.ending(w, mainH)
	case m.focus == 0:
		body = m.source(w, mainH)
	default:
		context := m.executionContext(w)
		body = strings.Join(context, "\n") + "\n" + m.dataFirstPanel(w, mainH-len(context))
	}
	n := len(m.replay.Run.Snapshots)
	pos := fmt.Sprintf("快照 %d/%d", m.replay.Position+1, n)
	if m.replay.End() {
		pos = fmt.Sprintf("END · %d 次暂停", n)
	}
	if m.navLabel != "" {
		pos += " · " + m.navLabel
	}
	pos += " " + timelineBar(m.replay.Position, n, min(16, w/6))
	help := "Home/End 首尾  R 重跑  Q 退出"
	if m.demo || m.offline {
		help = "Home/End 首尾  R 重置  Q 退出"
	}
	if m.focus == 1 && !m.replay.End() {
		help = "D 详情  C 源码  I 输入输出  S 栈帧  ? 帮助"
		if m.record != nil && !m.offline && !m.demo {
			help = "W 导出记录  D 详情  C 源码  I 输入输出  ? 帮助"
		}
	}
	if m.focus == 2 {
		help = "Enter 切换输入输出  Esc 返回数据  ? 帮助"
	}
	if m.focus == 0 && !m.replay.End() {
		help = "H 行历史  S 栈帧  Esc 返回数据  ? 帮助"
	}
	if m.standalone && m.focus == 0 {
		help += "  E 修改配置"
	}
	if m.replay.End() && m.record != nil && !m.offline && !m.demo {
		help = "W 导出记录  I 输入输出  R 重跑  Q 退出"
	}
	if m.notice != "" {
		help = m.notice
	}
	runHint := "R 重跑"
	if m.demo || m.offline {
		runHint = "R 重置"
	}
	return accent.Render(clip(title, w)) + "\n" + body + "\n" + muted.Render(clip(pos+"  ·  "+reasonLabel(m.replay.Run.Reason), w)) + "\n" + muted.Render(clip("←/→ 回放  ↑/↓ 选择  Tab 视图  F 筛选  J 跳转  "+runHint, w)) + "\n" + muted.Render(clip(help, w))
}
