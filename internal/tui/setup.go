package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/Routilet/c-observer/internal/observer"
)

type textField struct {
	text   string
	cursor int // rune index
}

func newField(s string) textField { return textField{text: s, cursor: len([]rune(s))} }
func (f *textField) edit(msg tea.KeyMsg, multiline bool) {
	r := []rune(f.text)
	p := max(0, min(f.cursor, len(r)))
	insert := func(s string) {
		var clean []rune
		s = strings.ReplaceAll(s, "\r\n", "\n")
		for _, c := range s {
			if c == '\n' && multiline || c == '\t' && multiline || !unicode.IsControl(c) {
				clean = append(clean, c)
			}
		}
		f.text = string(r[:p]) + string(clean) + string(r[p:])
		f.cursor = p + len(clean)
	}
	switch msg.Type {
	case tea.KeyLeft:
		p = max(0, p-1)
	case tea.KeyRight:
		p = min(len(r), p+1)
	case tea.KeyHome:
		for p > 0 && r[p-1] != '\n' {
			p--
		}
	case tea.KeyEnd:
		for p < len(r) && r[p] != '\n' {
			p++
		}
	case tea.KeyUp, tea.KeyDown:
		start := p
		for start > 0 && r[start-1] != '\n' {
			start--
		}
		col := p - start
		if msg.Type == tea.KeyUp && start > 0 {
			end := start - 1
			start = end
			for start > 0 && r[start-1] != '\n' {
				start--
			}
			p = min(start+col, end)
		} else if msg.Type == tea.KeyDown {
			end := p
			for end < len(r) && r[end] != '\n' {
				end++
			}
			if end < len(r) {
				start = end + 1
				end = start
				for end < len(r) && r[end] != '\n' {
					end++
				}
				p = min(start+col, end)
			}
		}
	case tea.KeyBackspace:
		if p > 0 {
			f.text = string(r[:p-1]) + string(r[p:])
			f.cursor = p - 1
			return
		}
	case tea.KeyDelete:
		if p < len(r) {
			f.text = string(r[:p]) + string(r[p+1:])
			f.cursor = p
			return
		}
	case tea.KeyCtrlU:
		f.text = ""
		f.cursor = 0
		return
	case tea.KeyEnter:
		if multiline {
			insert("\n")
			return
		}
	case tea.KeySpace:
		insert(" ")
		return
	case tea.KeyRunes:
		insert(string(msg.Runes))
		return
	}
	f.cursor = p
}

type SetupState struct {
	path, input, expected textField
	withExpected          bool
	focus                 int // path, stdin, expected checkbox, expected text, start
	message               string
	importing             bool
	importTarget          int
	importPath            textField
}

// NewSetup starts with an editable launch form for standalone use.
// NewLive skips it when another tool has already supplied a launch configuration.
func NewSetup(engine observer.Engine, config observer.LaunchConfig) Model {
	m := NewLive(engine, config)
	m.standalone = true
	m.settingUp = true
	m.setup = setupFrom(config)
	return m
}

// NewSetupPreview exposes the form without a live engine.
func NewSetupPreview(f Fixture) Model {
	m := NewPreview(f)
	m.standalone = true
	m.settingUp = true
	m.config.SourcePath = "main.c"
	m.setup = setupFrom(m.config)
	return m
}

func setupFrom(config observer.LaunchConfig) SetupState {
	s := SetupState{path: newField(config.SourcePath), input: newField(string(config.Stdin))}
	if config.ExpectedStdout != nil {
		s.withExpected = true
		s.expected = newField(string(*config.ExpectedStdout))
	}
	return s
}

func (s *SetupState) move(delta int) {
	s.focus = (s.focus + delta + 5) % 5
	if s.focus == 3 && !s.withExpected {
		s.focus = (s.focus + delta + 5) % 5
	}
}

func (s SetupState) launchConfig(base observer.LaunchConfig) (observer.LaunchConfig, error) {
	path := expandHome(strings.TrimSpace(s.path.text))
	if path == "" {
		return base, fmt.Errorf("请输入 C 源文件路径")
	}
	if !filepath.IsAbs(path) && base.WorkDir != "" {
		path = filepath.Join(base.WorkDir, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return base, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return base, fmt.Errorf("无法读取源文件：%s", SafeText(path, false))
	}
	// Standalone source changes must not reuse old source maps or array pages.
	oldPath := base.SourcePath
	if oldPath != "" && !filepath.IsAbs(oldPath) && base.WorkDir != "" {
		oldPath = filepath.Join(base.WorkDir, oldPath)
	}
	oldPath, _ = filepath.Abs(oldPath)
	if oldPath != path {
		base.ArrayRanges = nil
		base.SourceMap = nil
		base.KeepLocations = nil
		base.SkipSourcePaths = nil
		base.VisibleSourcePaths = []string{path}
	}
	base.SourcePath = path
	if base.WorkDir == "" || oldPath != path {
		base.WorkDir = filepath.Dir(path)
	}
	base.Stdin = []byte(s.input.text)
	base.ExpectedStdout = nil
	if s.withExpected {
		v := []byte(s.expected.text)
		base.ExpectedStdout = &v
	}
	return base, nil
}

func (m *Model) setupKey(msg tea.KeyMsg) tea.Cmd {
	s := &m.setup
	s.message = ""
	if s.importing {
		return m.importKey(msg)
	}
	switch msg.String() {
	case "esc":
		if m.replay.Run.Reason != "" && !m.demo {
			m.settingUp = false
			return nil
		}
		return tea.Quit
	case "ctrl+c":
		return tea.Quit
	case "ctrl+f":
		if s.focus == 0 {
			s.completePath(m.config.WorkDir)
		}
		return nil
	case "ctrl+o":
		if s.focus == 1 || s.focus == 3 {
			s.importing = true
			s.importTarget = s.focus
			s.importPath = newField("")
			return nil
		}
		s.message = "请先选择样例输入或预期输出。"
		return nil
	case "tab":
		s.move(1)
		return nil
	case "shift+tab":
		s.move(-1)
		return nil
	case "ctrl+r":
		return m.launchSetup()
	case "enter":
		switch s.focus {
		case 0:
			s.move(1)
			return nil
		case 2:
			s.withExpected = !s.withExpected
			return nil
		case 4:
			return m.launchSetup()
		}
	case " ":
		if s.focus == 2 {
			s.withExpected = !s.withExpected
			return nil
		}
	}
	switch s.focus {
	case 0:
		s.path.edit(msg, false)
	case 1:
		s.input.edit(msg, true)
	case 3:
		s.expected.edit(msg, true)
	case 2, 4:
		if msg.Type == tea.KeyUp {
			s.move(-1)
		}
		if msg.Type == tea.KeyDown {
			s.move(1)
		}
	}
	return nil
}
func (m *Model) launchSetup() tea.Cmd {
	if m.engine == nil {
		m.setup.message = "仅预览配置界面；当前未连接真实观察引擎。"
		return nil
	}
	cfg, err := m.setup.launchConfig(m.config)
	if err != nil {
		m.setup.message = err.Error()
		return nil
	}
	m.config = cfg
	m.settingUp = false
	return m.start()
}

func fieldLines(f textField, active bool, w, h int) []string {
	r := []rune(f.text)
	p := max(0, min(f.cursor, len(r)))
	text := SafeText(f.text, true)
	if active {
		text = SafeText(string(r[:p]), true) + "▏" + SafeText(string(r[p:]), true)
	}
	if text == "" {
		text = "（空）"
	}
	all := strings.Split(runewidth.Wrap(text, max(1, w)), "\n")
	start := 0
	if active {
		prefix := runewidth.Wrap(SafeText(string(r[:p]), true)+"▏", max(1, w))
		line := strings.Count(prefix, "\n")
		start = max(0, min(line-h+1, max(0, len(all)-h)))
	}
	out := []string{}
	for _, line := range all[start:min(len(all), start+h)] {
		out = append(out, clip(line, w))
	}
	for len(out) < h {
		out = append(out, "")
	}
	return out
}

func (m Model) setupView(w, h int) string {
	s := m.setup
	if s.importing {
		return m.importView(w, h)
	}
	lines := []string{accent.Render(clip("C Observer / 启动配置", w))}
	if m.demo {
		lines[0] = accent.Render(clip("C Observer / 合成预览 · 启动配置", w))
	}
	label := func(i int, text string) string {
		mark := "  "
		if s.focus == i {
			mark = "> "
		}
		return head(mark+text, s.focus == i, w)
	}
	lines = append(lines, label(0, "源文件路径（.c） · Ctrl+F 补全"))
	lines = append(lines, fieldLines(s.path, s.focus == 0, w, 1)...)
	fieldH := max(2, min(5, (h-10)/2))
	lines = append(lines, label(1, "样例输入 · Enter 换行 / Ctrl+O 导入"))
	lines = append(lines, fieldLines(s.input, s.focus == 1, w, fieldH)...)
	check := "[ ]"
	if s.withExpected {
		check = "[x]"
	}
	lines = append(lines, label(2, check+" 提供预期输出（可选）"))
	if s.withExpected {
		lines = append(lines, label(3, "预期输出 · Enter 换行 / Ctrl+O 导入"))
		lines = append(lines, fieldLines(s.expected, s.focus == 3, w, fieldH)...)
	}
	lines = append(lines, label(4, "开始观察"))
	if s.message != "" {
		lines = append(lines, warning.Render(clip(SafeText(s.message, false), w)))
	}
	lines = append(lines, muted.Render(clip("Tab 切换  Ctrl+R 开始  Esc 退出", w)))
	return fixed(lines, w, h)
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		if home, err := os.UserHomeDir(); err == nil {
			if len(path) == 1 {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
func resolvePath(base, path string) string {
	path = expandHome(path)
	if !filepath.IsAbs(path) && base != "" {
		return filepath.Join(base, path)
	}
	return path
}
func (s *SetupState) completePath(base string) {
	text := expandHome(s.path.text)
	dir, prefix := filepath.Split(text)
	resolved := resolvePath(base, dir)
	if resolved == "" {
		resolved = "."
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		s.message = "无法列出路径：" + SafeText(err.Error(), false)
		return
	}
	var matches []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && (e.IsDir() || strings.EqualFold(filepath.Ext(e.Name()), ".c")) {
			matches = append(matches, e.Name())
		}
	}
	if len(matches) == 0 {
		s.message = "没有匹配的 C 源文件或目录。"
		return
	}
	common := []rune(matches[0])
	for _, name := range matches[1:] {
		r := []rune(name)
		n := 0
		for n < len(common) && n < len(r) && common[n] == r[n] {
			n++
		}
		common = common[:n]
	}
	completed := dir + string(common)
	if len(matches) == 1 {
		if info, err := os.Stat(resolvePath(base, completed)); err == nil && info.IsDir() {
			completed += string(os.PathSeparator)
		}
	}
	s.path = newField(completed)
	if len(matches) > 1 {
		s.message = fmt.Sprintf("%d 个匹配项：%s", len(matches), strings.Join(matches[:min(3, len(matches))], "  "))
	}
}

const maxImportedSample = 1 << 20

func readSample(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("请选择普通文件")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("请选择普通文件")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxImportedSample+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxImportedSample {
		return "", fmt.Errorf("样例文件超过 1 MiB 限制")
	}
	return string(data), nil
}
func (m *Model) importKey(msg tea.KeyMsg) tea.Cmd {
	s := &m.setup
	switch msg.String() {
	case "esc":
		s.importing = false
	case "ctrl+c":
		return tea.Quit
	case "enter":
		path := strings.TrimSpace(s.importPath.text)
		if path == "" {
			s.message = "请输入样例文件路径"
			return nil
		}
		text, err := readSample(resolvePath(m.config.WorkDir, path))
		if err != nil {
			s.message = "导入失败：" + SafeText(err.Error(), false)
			return nil
		}
		if s.importTarget == 1 {
			s.input = newField(text)
		} else {
			s.expected = newField(text)
			s.withExpected = true
		}
		s.importing = false
		s.message = "已导入样例文件（原始字节）"
	default:
		s.importPath.edit(msg, false)
	}
	return nil
}
func (m Model) importView(w, h int) string {
	s := m.setup
	lines := []string{accent.Render("C Observer / 导入样例"), "文件路径（相对工作目录或绝对路径）："}
	lines = append(lines, fieldLines(s.importPath, true, w, 2)...)
	if s.message != "" {
		lines = append(lines, warning.Render(clip(s.message, w)))
	}
	lines = append(lines, muted.Render("Enter 导入 · Esc 返回"))
	return fixed(lines, w, h)
}
