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
}
type runDone struct{ result observer.RunResult }
type clockTick time.Time

func NewPreview(f Fixture) Model {
	m := Model{replay:Replay{Run:f.Run},demo:true, width:100,height:30,collapsed:map[string]bool{}}
	m.config.Stdin=[]byte(f.Stdin)
	if f.ExpectedStdout!=nil { b:=[]byte(*f.ExpectedStdout); m.config.ExpectedStdout=&b }
	return m
}

// NewLive is the integration boundary; the engine is injected by the caller.
func NewLive(engine observer.Engine, config observer.LaunchConfig) Model {
	return Model{engine:engine,config:config,width:100,height:30,collapsed:map[string]bool{}}
}

func (m Model) Init() tea.Cmd {
	if m.demo || m.engine==nil { return nil }
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
		return m,m.start()
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
	case tea.KeyMsg:
		k:=msg.String()
		if m.running {
			if k=="esc" || k=="ctrl+c" || k=="q" {
				m.stopping=true
				if k=="ctrl+c" || k=="q" {m.quitting=true}
				if m.cancel!=nil {m.cancel()}
			}
			return m,nil
		}
		switch k {
		case "esc","q","ctrl+c": return m,tea.Quit
		case "r":
			if m.demo {m.replay.Seek(0);m.resetScroll();m.collapsed=map[string]bool{};break}
			return m,m.start()
		case "left": m.replay.Move(-1);m.resetScroll()
		case "right": m.replay.Move(1);m.resetScroll()
		case "home": m.replay.Seek(0);m.resetScroll()
		case "end": m.replay.Seek(len(m.replay.Run.Snapshots));m.resetScroll()
		case "tab": m.focus=(m.focus+1)%3
		case "shift+tab": m.focus=(m.focus+2)%3
		case "up","down","pgup","pgdown":
			d:=1
			if k=="up" || k=="pgup" {d=-1}
			if k=="pgup" || k=="pgdown" {d*=max(1,m.height/3)}
			switch m.focus {
			case 0: m.sourceScroll+=d
			case 1: m.varCursor=max(0,min(m.varCursor+d,len(m.replay.Rows(m.collapsed))-1))
			case 2: m.consoleScroll=max(0,m.consoleScroll+d)
			}
		case "enter":
			if m.focus==1 {
				rows:=m.replay.Rows(m.collapsed)
				if m.varCursor>=0 && m.varCursor<len(rows) && rows[m.varCursor].Expandable {
					key:=rows[m.varCursor].Key
					// Copy map to keep Bubble Tea value updates isolated.
					next:=make(map[string]bool,len(m.collapsed)+1)
					for k,v:=range m.collapsed {next[k]=v}
					next[key]=!next[key];m.collapsed=next
				}
			}
			if m.focus==2 {m.consoleTab=(m.consoleTab+1)%4;m.consoleScroll=0}
		}
	}
	return m,nil
}
func (m *Model) resetScroll() {m.sourceScroll,m.varCursor,m.varScroll,m.consoleScroll=0,0,0,0}

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
	lines:=[]string{head(" SOURCE",m.focus==0,w)}
	if s==nil {return fixed(append(lines,muted.Render("No active source position.")),w,h)}
	loc:=s.Location
	lines=append(lines,muted.Render(clip(SafeText(fmt.Sprintf("%s:%d  %s",loc.File,loc.Line,loc.Function),false),w)))
	src:=sourceFor(m.replay.Run,loc.File)
	if loc.File=="" || src=="" || loc.Line<=0 {
		return fixed(append(lines,clip("Unmapped: "+SafeText(loc.ReportedFile,false),w)),w,h)
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
	lines:=[]string{head(" VARIABLES",m.focus==1,w)}
	s:=m.replay.Current()
	if s==nil {return fixed(append(lines,muted.Render("No active variable state.")),w,h)}
	label:="Active frame"
	if len(s.Frames)>0 {
		label=SafeText(s.Frames[0].Location.Function,false)
		if !s.Frames[0].VariablesComplete {label+=" · partial list"}
	}
	if len(s.Globals)>0 {label+=" + globals"}
	if !s.GlobalsComplete && m.replay.Run.Session.Capabilities.Globals {label+=" · globals partial"}
	lines=append(lines,muted.Render(clip(label,w)))
	rows:=m.replay.Rows(m.collapsed)
	if len(rows)==0 {return fixed(append(lines,muted.Render("No recorded variables.")),w,h)}
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
		typeText:=" ["+r.Type+"]"
		left:=clip(prefix,max(1,w/2))
		rest:=max(0,w-runewidth.StringWidth(left)-1)
		line:=left+" "+clip(value+typeText,rest)
		if r.Changed {line=changed.Render(line)}
		if m.focus==1 && i==m.varCursor {line=selected.Render(line)}
		lines=append(lines,line)
	}
	return fixed(lines,w,h)
}
func (m Model) console(w,h int) string {
	labels:=[]string{"INPUT","STDOUT","STDERR","EXPECTED"}
	heading:=" "+labels[m.consoleTab]+"  [Enter: switch]"
	lines:=[]string{head(heading,m.focus==2,w)}
	out,errOut:=m.replay.Output()
	var data string
	switch m.consoleTab {
	case 0:data=string(m.config.Stdin)
	case 1:data=string(out)
	case 2:data=string(errOut)
	case 3:if m.config.ExpectedStdout!=nil {data=string(*m.config.ExpectedStdout)} else {data="(not supplied)"}
	}
	if data=="" {data="(empty)"}
	if m.consoleTab==1 && m.replay.Run.Output.StdoutTruncated || m.consoleTab==2 && m.replay.Run.Output.StderrTruncated {data+="\n[output truncated during run]"}
	data=SafeText(data,true)
	a:=strings.Split(data,"\n")
	start:=max(0,min(m.consoleScroll,max(0,len(a)-1)))
	for _,l:=range a[start:min(len(a),start+max(0,h-1))] {lines=append(lines,clip(l,w))}
	return fixed(lines,w,h)
}
func (m Model) ending(w,h int) string {
	r:=m.replay.Run
	status:=fmt.Sprintf("END · %s",r.Reason)
	if r.ExitCode!=nil {status+=fmt.Sprintf(" · exit %d",*r.ExitCode)}
	st:=warning
	if r.Reason==observer.Exited {st=accent}
	if r.Reason==observer.CompileFailed || r.Reason==observer.BackendFailed || r.Reason==observer.Signaled {st=failure}
	lines:=[]string{st.Render(clip(status,w)),muted.Render(clip(fmt.Sprintf("%d recorded pauses; no active source or variables.",len(r.Snapshots)),w))}
	if r.Message!="" {lines=append(lines,clip(SafeText(r.Message,false),w))}
	if r.CleanupError!="" {lines=append(lines,failure.Render(clip("Cleanup: "+SafeText(r.CleanupError,false),w)))}
	if m.config.ExpectedStdout!=nil && r.Reason==observer.Exited && !r.Output.StdoutTruncated {
		match:="Final stdout differs from expected (exact bytes)."
		if string(r.Output.Stdout)==string(*m.config.ExpectedStdout) {match="Final stdout matches expected (exact bytes)."}
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
	if w<45 || h<14 {return "Resize terminal to at least 45 × 14. Q: quit."}
	title:="C Observer"
	if m.demo {title+=" · SYNTHETIC PREVIEW"}
	if m.running {
		status:="Compiling and recording..."
		if m.stopping {status="Stopping; waiting for debugger cleanup..."}
		return accent.Render(title)+"\n\n"+status+"\n"+time.Since(m.started).Round(100*time.Millisecond).String()+"\n\nEsc: cancel · Q: cancel and exit"
	}
	if m.err!="" {return failure.Render("Invalid record: "+SafeText(m.err,false))+"\nR: retry · Q: quit"}
	// Header/footer consume three rows. The console is kept compact on wide terminals.
	mainH:=h-3
	var body string
	if w>=86 {
		consoleH:=max(4,min(7,mainH/3))
		topH:=mainH-consoleH
		if m.replay.End() {body=m.ending(w,topH)} else {
			left:=w*55/100
			right:=w-left-1
			body=lipgloss.JoinHorizontal(lipgloss.Top,m.source(left,topH),muted.Render(strings.Repeat("│\n",topH-1)+"│"),m.variables(right,topH))
		}
		body+="\n"+m.console(w,consoleH)
	} else if m.focus==2 {body=m.console(w,mainH)} else if m.replay.End() {body=m.ending(w,mainH)} else if m.focus==0 {body=m.source(w,mainH)} else {body=m.variables(w,mainH)}
	n:=len(m.replay.Run.Snapshots)
	pos:=fmt.Sprintf("Pause %d/%d",m.replay.Position+1,n)
	if m.replay.End() {pos=fmt.Sprintf("END · %d pauses",n)}
	help:="←/→ step  ↑/↓ scroll  Tab focus  Enter expand/switch  Home/End  R rerun  Q quit"
	if m.demo {help=strings.Replace(help,"R rerun","R reset",1)}
	return accent.Render(clip(title,w))+"\n"+body+"\n"+muted.Render(clip(pos+"  ·  "+string(m.replay.Run.Reason),w))+"\n"+muted.Render(clip(help,w))
}
