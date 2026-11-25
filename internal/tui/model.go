package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/Annouimo/c-observer/internal/observer"
)

type Model struct {
	replay Replay
	config observer.LaunchConfig
	engine observer.Engine
	demo bool
	width, height int
	focus, sourceScroll, varCursor, varScroll, consoleScroll int
	consoleTab int
	collapsed map[string]bool
	running, stopping, quitting bool
	cancel context.CancelFunc
	started time.Time
	err string
	mode, notice string
	standalone, settingUp bool
	setup SetupState
	target VariableRow
	targetPosition, modalScroll, modalCol int
	history []HistoryEntry
	rangeInput [2]string
	rangeField int
}
type runDone struct{ result observer.RunResult }
type clockTick time.Time

func NewPreview(f Fixture) Model {
	m := Model{replay:Replay{Run:f.Run},demo:true, width:100,height:30,consoleTab:1,collapsed:map[string]bool{}}
	m.config.Stdin=[]byte(f.Stdin)
	if f.ExpectedStdout!=nil { b:=[]byte(*f.ExpectedStdout); m.config.ExpectedStdout=&b }
	return m
}

// NewLive is the integration boundary; the engine is injected by the caller.
func NewLive(engine observer.Engine, config observer.LaunchConfig) Model {
	return Model{engine:engine,config:config,width:100,height:30,consoleTab:1,collapsed:map[string]bool{}}
}

func (m Model) Init() tea.Cmd {
	if m.demo || m.engine==nil || m.settingUp { return nil }
	return func() tea.Msg { return startRun{} }
}
type startRun struct{}
func tick() tea.Cmd { return tea.Tick(150*time.Millisecond,func(t time.Time) tea.Msg {return clockTick(t)}) }
func (m *Model) start() tea.Cmd {
	if m.engine==nil || m.running {return nil}
	ctx,cancel:=context.WithCancel(context.Background())
	m.cancel=cancel
	m.started=time.Now()
	m.running=true
	m.stopping=false
	m.err=""
	engine,cfg:=m.engine,m.config
	return tea.Batch(tick(),func() tea.Msg { return runDone{engine.Run(ctx,cfg)} })
}

func (m Model) Update(msg tea.Msg) (tea.Model,tea.Cmd) {
	switch msg:=msg.(type) {
	case tea.WindowSizeMsg:
		m.width,m.height=msg.Width,msg.Height
	case startRun:
		cmd:=m.start();return m,cmd
	case clockTick:
		if m.running { return m,tick() }
	case runDone:
		if m.cancel!=nil { m.cancel();m.cancel=nil }
		m.running=false
		m.stopping=false
		if m.quitting {return m,tea.Quit}
		if err:=ValidateRun(msg.result);err!=nil {m.err=err.Error();return m,nil}
		m.replay=Replay{Run:msg.result}
		m.sourceScroll,m.varCursor,m.varScroll,m.consoleScroll=0,0,0,0
		m.collapsed=map[string]bool{}
		m.mode,m.notice="",""
		if len(msg.result.Snapshots)==0 {m.consoleTab=1}
	case tea.KeyMsg:
		if m.settingUp {cmd:=m.setupKey(msg);return m,cmd}
		k:=msg.String()
		if m.running {
			if k=="esc" || k=="ctrl+c" || k=="q" {
				m.stopping=true
				if k=="ctrl+c" || k=="q" {m.quitting=true}
				if m.cancel!=nil {m.cancel()}
			}
			return m,nil
		}
		if m.mode!="" {cmd:=m.handleInspector(msg);return m,cmd}
		m.notice=""
		switch k {
		case "d","h","a":
			if k=="h" && m.focus==0 {
				if snap:=m.replay.Current();snap!=nil && snap.Location.File!="" && snap.Location.Line>0 {
					m.mode="line-history"
					m.target=VariableRow{Path:fmt.Sprintf("%s:%d",snap.Location.File,snap.Location.Line)}
					m.history=m.replay.LineHistory(snap.Location)
					m.modalScroll=0
				}
				break
			}
			if m.focus==1 && !m.replay.End() {
				mode:=map[string]string{"d":"detail","h":"history","a":"range"}[k]
				m.openInspector(mode)
				m.targetPosition=m.replay.Position
			}
		case "esc","q","ctrl+c": return m,tea.Quit
		case "e":
			if m.standalone {m.setup=setupFrom(m.config);m.settingUp=true}
		case "r":
			if m.demo {m.replay.Seek(0);m.resetScroll();m.collapsed=map[string]bool{};break}
			cmd:=m.start();return m,cmd
		case "left": m.seek(m.replay.Position-1)
		case "right": m.seek(m.replay.Position+1)
		case "home": m.seek(0)
		case "end": m.seek(len(m.replay.Run.Snapshots))
		case "tab": m.focus=(m.focus+1)%3
		case "shift+tab": m.focus=(m.focus+2)%3
		case "up","down","pgup","pgdown":
			d:=1
			if k=="up" || k=="pgup" {d=-1}
			if k=="pgup" || k=="pgdown" {d*=max(1,m.height/3)}
			switch m.focus {
			case 0: m.sourceScroll+=d; m.clampSourceScroll()
			case 1: m.varCursor=max(0,min(m.varCursor+d,len(m.replay.Rows(m.collapsed))-1))
			case 2: m.consoleScroll=max(0,min(m.consoleScroll+d,len(m.consoleLines(m.consoleTab,m.consoleWidth()))-1))
			}
		case "enter":
			if m.focus==1 {
				rows:=m.replay.Rows(m.collapsed)
				if m.varCursor>=0 && m.varCursor<len(rows) && rows[m.varCursor].Expandable {
					key:=rows[m.varCursor].Key
					// Copy map to keep Bubble Tea value updates isolated.
					next:=make(map[string]bool,len(m.collapsed)+1)
					for k,v:=range m.collapsed {next[k]=v}
					next[key]=!rows[m.varCursor].Collapsed;m.collapsed=next
				}
			}
			if m.focus==2 {m.consoleTab=(m.consoleTab+1)%4;m.consoleScroll=0}
		}
	}
	return m,nil
}
func (m *Model) clampSourceScroll() {
	if m.replay.End() {
		m.sourceScroll=max(0,min(m.sourceScroll,len(m.replay.Run.Diagnostics)+5))
		return
	}
	if s:=m.replay.Current();s!=nil {
		n:=max(1,m.height-6)
		if m.width>=86 {n-=max(4,min(7,(m.height-4)/3))}
		n=max(1,n)
		total:=len(strings.Split(sourceFor(m.replay.Run,s.Location.File),"\n"))
		base:=s.Location.Line-1-n/2
		m.sourceScroll=max(-base,min(m.sourceScroll,max(0,total-n)-base))
	}
}
func (m *Model) seek(pos int) {
	rows:=m.replay.Rows(m.collapsed)
	var key string
	if m.varCursor>=0 && m.varCursor<len(rows) && rows[m.varCursor].Reliable {key=rows[m.varCursor].Key}
	m.replay.Seek(pos)
	m.resetScroll()
	if key!="" {
		for i,row:=range m.replay.Rows(m.collapsed) {if row.Key==key {m.varCursor=i;break}}
	}
	if m.replay.End() {m.consoleTab=1}
}
func (m *Model) resetScroll() {
	m.sourceScroll,m.varScroll,m.consoleScroll=0,0,0
	rows:=m.replay.Rows(m.collapsed)
	m.varCursor=max(0,min(m.varCursor,len(rows)-1))
}

func reasonLabel(reason observer.FinishReason) string {
	switch reason {
	case observer.Exited:return "正常退出"
	case observer.NonzeroExit:return "非零退出"
	case observer.Signaled:return "异常终止"
	case observer.TimedOut:return "运行超时"
	case observer.RequestTimedOut:return "调试请求超时"
	case observer.InvalidConfig:return "配置错误"
	case observer.Cancelled:return "已取消"
	case observer.LimitReached:return "达到采集上限"
	case observer.CompileFailed:return "编译失败"
	case observer.BackendFailed:return "调试器错误"
	default:return SafeText(string(reason),false)
	}
}

var (
	muted=lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light:"240",Dark:"245"})
	accent=lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light:"25",Dark:"75"}).Bold(true)
	changed=lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light:"28",Dark:"114"})
	warning=lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light:"130",Dark:"221"})
	failure=lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light:"124",Dark:"203"})
	selected=lipgloss.NewStyle().Bold(true).Underline(true)
)
func clip(s string,w int) string {
	if w<=0{return ""}
	if runewidth.StringWidth(s)<=w{return s}
	return runewidth.Truncate(s,w,"…")
}
func pad(s string,w int) string {return s+strings.Repeat(" ",max(0,w-lipgloss.Width(s)))}
func fixed(lines []string,w,h int) string {
	if len(lines)>h {lines=lines[:h]}
	for len(lines)<h {lines=append(lines,"")}
	for i:=range lines {lines[i]=pad(lines[i],w)}
	return strings.Join(lines,"\n")
}
func head(s string,focused bool,w int) string {
	s=clip(s,w)
	if focused {return accent.Render(s)}
	return muted.Render(s)
}
func sourceFor(run observer.RunResult,path string) string {
	for _,s:=range run.Session.Sources {if s.Path==path{return s.Text}}
	return ""
}
func (m Model) source(w,h int) string {
	s:=m.replay.Current()
	lines:=[]string{head(" 源代码",m.focus==0,w)}
	if s==nil {return fixed(append(lines,muted.Render("没有当前源码位置。")),w,h)}
	loc:=s.Location
	lines=append(lines,muted.Render(clip(SafeText(fmt.Sprintf("%s:%d  %s",loc.File,loc.Line,loc.Function),false),w)))
	src:=sourceFor(m.replay.Run,loc.File)
	if loc.File=="" || src=="" || loc.Line<=0 {
		return fixed(append(lines,clip("未映射: "+SafeText(loc.ReportedFile,false),w)),w,h)
	}
	all:=strings.Split(SafeText(src,true),"\n")
	n:=max(1,h-2)
	start:=max(0,min(loc.Line-1-n/2+m.sourceScroll,max(0,len(all)-n)))
	// Lex the whole file so multiline comments remain highlighted when scrolled.
	tokens:=lexC(strings.Join(all,"\n"))
	for i:=start;i<min(len(all),start+n);i++ {
		mark:=" "
		if i+1==loc.Line {mark=">"}
		prefix:=fmt.Sprintf("%s%4d ",mark,i+1)
		if i+1==loc.Line {prefix=accent.Render(prefix)} else {prefix=muted.Render(prefix)}
		lines=append(lines,prefix+renderTokens(tokens[i],max(0,w-6)))
	}
	return fixed(lines,w,h)
}
func (m Model) variables(w,h int) string {
	lines:=[]string{head(" 变量",m.focus==1,w)}
	s:=m.replay.Current()
	if s==nil {return fixed(append(lines,muted.Render("没有当前变量状态。")),w,h)}
	label:="当前栈帧"
	if len(s.Frames)>0 {
		label=SafeText(s.Frames[0].Location.Function,false)
		if !s.Frames[0].VariablesComplete {label+=" · 列表不完整"}
	}
	if len(s.Globals)>0 {label+=" + 全局变量"}
	if !s.GlobalsComplete && m.replay.Run.Session.Capabilities.Globals {label+=" · 全局列表不完整"}
	lines=append(lines,muted.Render(clip(label+" · D 数据表 / H 历史",w)))
	rows:=m.replay.Rows(m.collapsed)
	allRows:=m.replay.Rows(nil)
	if len(rows)==0 {return fixed(append(lines,muted.Render("无已记录变量。")),w,h)}
	n:=max(1,h-2)
	start:=max(0,min(m.varCursor-n+1,max(0,len(rows)-n)))
	for i:=start;i<min(len(rows),start+n);i++ {
		r:=rows[i]
		cursor:=" "
		if m.focus==1 && i==m.varCursor {cursor=">"}
		toggle:=" "
		if r.Expandable {toggle="-";if r.Collapsed {toggle="+"}}
		prefix:=cursor+strings.Repeat("  ",min(r.Depth,6))+toggle+" "+r.Name
		value:=r.Value
		if r.Detail!="" {if value!="" {value+=" · "};value+=r.Detail}
		if r.Changed {value="* "+r.Before+" → "+r.Value}
		if r.Raw.Kind==observer.Array && len(r.Raw.Children)>0 && r.Raw.Children[0].Index!=nil {value+=fmt.Sprintf(" · [%d…]",*r.Raw.Children[0].Index)}
		hiddenChanges:=0
		if r.Collapsed {
			for _,child:=range allRows {if child.Changed && strings.HasPrefix(child.Key,r.Key+"/") {hiddenChanges++}}
			if hiddenChanges>0 {value=fmt.Sprintf("* %d 项变化 · %s",hiddenChanges,value)}
		}
		typeText:=" ["+r.Type+"]"
		nameWidth:=max(1,w*37/100)
		left:=pad(clip(prefix,nameWidth),nameWidth)
		rest:=max(0,w-nameWidth-1)
		line:=left+" "+clip(value+typeText,rest)
		if r.Changed || hiddenChanges>0 {line=changed.Render(line)}
		if m.focus==1 && i==m.varCursor {line=selected.Render(line)}
		lines=append(lines,line)
	}
	return fixed(lines,w,h)
}
func (m Model) consoleText() string {return m.consoleTextFor(m.consoleTab)}
func (m Model) consoleTextFor(tab int) string {
	out,errOut:=m.replay.Output()
	var data string
	switch tab {
	case 0:data=string(m.config.Stdin)
	case 1:data=string(out)
	case 2:data=string(errOut)
	case 3:if m.config.ExpectedStdout!=nil {data=string(*m.config.ExpectedStdout)} else {data="（未提供）"}
	}
	if data=="" {data="（空）"}
	if tab==1 && m.replay.Run.Output.StdoutTruncated || tab==2 && m.replay.Run.Output.StderrTruncated {data+="\n[output truncated during run]"}
	return SafeText(data,true)
}
func (m Model) consoleWidth() int {
	if m.width<86 {return m.width}
	if m.consoleTab==0 {return m.width*55/100}
	return m.width-m.width*55/100-1
}
func (m Model) consoleLines(tab,w int) []string {
	return strings.Split(runewidth.Wrap(m.consoleTextFor(tab),max(1,w)),"\n")
}
func (m Model) console(w,h int) string {return m.consoleView(m.consoleTab,w,h,m.consoleScroll,m.focus==2)}
func (m Model) consoleView(tab,w,h,scroll int,focused bool) string {
	labels:=[]string{"输入","实际输出","错误流","预期输出"}
	lines:=[]string{head(" "+labels[tab],focused,w)}
	a:=m.consoleLines(tab,w)
	start:=max(0,min(scroll,max(0,len(a)-1)))
	for _,l:=range a[start:min(len(a),start+max(0,h-1))] {lines=append(lines,clip(l,w))}
	return fixed(lines,w,h)
}
func (m Model) ending(w,h int) string {
	r:=m.replay.Run
	status:=fmt.Sprintf("END · %s",reasonLabel(r.Reason))
	if r.ExitCode!=nil {status+=fmt.Sprintf(" · 退出码 %d",*r.ExitCode)}
	st:=warning
	if r.Reason==observer.Exited {st=accent}
	if r.Reason==observer.CompileFailed || r.Reason==observer.BackendFailed || r.Reason==observer.Signaled {st=failure}
	lines:=[]string{st.Render(clip(status,w)),muted.Render(clip(fmt.Sprintf("已记录 %d 次暂停；无活动源码位置或变量。",len(r.Snapshots)),w))}
	if r.Message!="" {lines=append(lines,clip(SafeText(r.Message,false),w))}
	if r.CleanupError!="" {lines=append(lines,failure.Render(clip("清理失败: "+SafeText(r.CleanupError,false),w)))}
	if m.config.ExpectedStdout!=nil && r.Reason==observer.Exited && !r.Output.StdoutTruncated {
		match:="实际输出与预期不同（逐字节比较）。"
		if string(r.Output.Stdout)==string(*m.config.ExpectedStdout) {match="实际输出与预期一致（逐字节比较）。"}
		lines=append(lines,clip(match,w))
	}
	for _,d:=range r.Diagnostics {
		at:=""
		if d.Location!=nil {at=fmt.Sprintf("%s:%d ",d.Location.File,d.Location.Line)}
		lines=append(lines,clip(SafeText(at+d.Severity+": "+d.Message,false),w))
	}
	start:=max(0,min(m.sourceScroll,max(0,len(lines)-h)))
	return fixed(lines[start:],w,h)
}
func (m Model) View() string {
	w,h:=m.width,m.height
	if w<45 || h<14 {return "请将终端调整至至少 45 × 14。Q 退出。"}
	if m.settingUp {return m.setupView(w,h)}
	title:="C Observer"
	if m.demo {title+=" · 合成数据预览"}
	if m.running {
		status:="正在编译并采集运行状态…"
		if m.stopping {status="正在停止，等待调试器清理…"}
		return accent.Render(title)+"\n\n"+status+"\n"+time.Since(m.started).Round(100*time.Millisecond).String()+"\n\nEsc 取消 · Q 取消并退出"
	}
	if m.err!="" {return failure.Render("运行记录无效: "+SafeText(m.err,false))+"\nR 重试 · Q 退出"}
	if m.mode!="" {return m.inspector(w,h)}
	// Header/footer consume three rows. The console is kept compact on wide terminals.
	mainH:=h-4
	var body string
	if w>=86 {
		consoleH:=max(4,min(7,mainH/3))
		topH:=mainH-consoleH
		if m.replay.End() {body=m.ending(w,topH)} else {
			left:=w*55/100
			right:=w-left-1
			body=lipgloss.JoinHorizontal(lipgloss.Top,m.source(left,topH),muted.Render(strings.Repeat("│\n",topH-1)+"│"),m.variables(right,topH))
		}
		inputW:=w*55/100
		outputW:=w-inputW-1
		inputScroll,outputScroll:=0,0
		outputTab:=m.consoleTab
		if m.consoleTab==0 {inputScroll=m.consoleScroll;outputTab=1} else {outputScroll=m.consoleScroll}
		body+="\n"+lipgloss.JoinHorizontal(lipgloss.Top,m.consoleView(0,inputW,consoleH,inputScroll,m.focus==2 && m.consoleTab==0),muted.Render(strings.Repeat("│\n",consoleH-1)+"│"),m.consoleView(outputTab,outputW,consoleH,outputScroll,m.focus==2 && m.consoleTab!=0))
	} else if m.focus==2 {body=m.console(w,mainH)} else if m.replay.End() {body=m.ending(w,mainH)} else if m.focus==0 {body=m.source(w,mainH)} else {body=m.variables(w,mainH)}
	n:=len(m.replay.Run.Snapshots)
	pos:=fmt.Sprintf("快照 %d/%d",m.replay.Position+1,n)
	if m.replay.End() {pos=fmt.Sprintf("END · %d 次暂停",n)}
	help:="Home/End 首尾  R 重跑  Q 退出"
	if m.demo {help="Home/End 首尾  R 重置  Q 退出"}
	if m.focus==1 && !m.replay.End() {help="D 数据表  H 历史  A 范围  R 重跑/重置  Q 退出"}
	if m.focus==2 {help="Enter 切换输入/实际输出/错误流/预期输出  Q 退出"}
	if m.focus==0 && !m.replay.End() {help+="  H 本行历史"}
	if m.standalone && m.focus==0 {help+="  E 修改配置"}
	if m.notice!="" {help=m.notice}
	return accent.Render(clip(title,w))+"\n"+body+"\n"+muted.Render(clip(pos+"  ·  "+reasonLabel(m.replay.Run.Reason),w))+"\n"+muted.Render(clip("←/→ 快照  ↑/↓ 选择  Tab 区域  Enter 展开/切换",w))+"\n"+muted.Render(clip(help,w))
}
