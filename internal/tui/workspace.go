package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"github.com/Annouimo/c-observer/internal/observer"
)

func timelineBar(pos,n,width int) string {
	if n<=0 || width<=0 {return ""}
	done:=min(width,max(0,pos*width/n))
	return "["+strings.Repeat("━",done)+strings.Repeat("─",width-done)+"]"
}
func (m Model) selectedVariable() (VariableRow,bool) {
	rows:=m.replay.Rows(m.collapsed)
	if m.varCursor<0 || m.varCursor>=len(rows) {return VariableRow{},false}
	return rows[m.varCursor],true
}
func (m Model) selectedLocation() observer.SourceLocation {
	loc:=m.replay.ViewedLocation()
	if m.sourceLine>0 {loc.Line=m.sourceLine}
	return loc
}
func (m *Model) moveSource(delta int) {
	if m.replay.End() {m.sourceScroll+=delta;m.clampSourceScroll();return}
	loc:=m.selectedLocation()
	if loc.File=="" {return}
	n:=len(strings.Split(sourceFor(m.replay.Run,loc.File),"\n"))
	m.sourceLine=max(1,min(max(1,loc.Line)+delta,n))
	m.sourceScroll=0
}
func skipTokenCells(tokens []token,n int) []token {
	if n<=0 {return tokens}
	var out []token
	for _,t:=range tokens {
		if n<=0 {out=append(out,t);continue}
		r:=[]rune(t.text)
		i:=0
		for i<len(r) && n>0 {n-=max(0,runewidth.RuneWidth(r[i]));i++}
		if i<len(r) {out=append(out,token{text:string(r[i:]),kind:t.kind})}
	}
	return out
}

func (m *Model) toggleTree() {
	r,ok:=m.selectedVariable()
	if !ok || !r.Expandable {return}
	next:=make(map[string]bool,len(m.collapsed)+1)
	for k,v:=range m.collapsed {next[k]=v}
	next[r.Key]=!r.Collapsed
	m.collapsed=next
}

func (m *Model) toggleFull(row VariableRow) {
	if row.Raw.Kind!=observer.Array {return}
	next:=map[string]bool{}
	for k,v:=range m.fullArrays {next[k]=v}
	next[row.Key]=!next[row.Key]
	m.fullArrays=next
}

func (m *Model) navigate(dir int) {
	if m.navLabel=="" {m.seek(m.replay.Position+dir);return}
	pos:=m.replay.Position
	if dir>0 {
		for _,n:=range m.navigation {if n>pos {m.seek(n);return}}
		if !m.replay.End() {m.seek(len(m.replay.Run.Snapshots));return}
	} else {
		for i:=len(m.navigation)-1;i>=0;i-- {if m.navigation[i]<pos {m.seek(m.navigation[i]);return}}
	}
	m.notice="此方向没有匹配快照。F 可恢复全部快照。"
}

func (m *Model) setFilter(i int) {
	m.mode=""
	if i==0 {m.navigation=nil;m.navLabel="";return}
	idx:=m.replay.historyIndex()
	var positions []int
	label:=""
	switch i {
	case 1:
		positions=idx.Changes;label="数据变化"
	case 2:
		row,ok:=m.selectedVariable()
		if !ok || !row.Reliable {m.notice="请先选择身份可靠的变量。";return}
		for _,e:=range idx.Objects[row.Key] {if e.Changed {positions=append(positions,e.Position)}}
		label=SafeText(row.Path,false)+" 的变化"
	case 3:
		loc:=m.selectedLocation()
		if loc.File=="" || loc.Line<=0 {m.notice="没有可选源码行。";return}
		for _,e:=range idx.Lines[lineKey{loc.File,loc.Line}] {positions=append(positions,e.Position)}
		label=fmt.Sprintf("第 %d 行",loc.Line)
	}
	if len(positions)==0 {m.notice="没有匹配的已记录快照，保持原导航模式。";return}
	m.navigation=positions;m.navLabel=label
}

func (m Model) workspaceMode() bool {
	switch m.mode {case "filter","stack","jump","help","object","display-range":return true}
	return false
}
func (m Model) menuOptions() []string {
	switch m.mode {
	case "filter":return []string{"全部快照","任意可靠数据变化","当前选中变量变化","当前选中源码行"}
	case "object":return []string{"查看数据详情","查看变化历史","仅导航此对象的变化","切换自动范围 / 全部已采集","手动设置展示范围","调整采集范围并重跑","展开 / 折叠树"}
	case "stack":
		var options []string
		if s:=m.replay.Current();s!=nil {
			for i,f:=range s.Frames {
				tag:="";if i==0 {tag=" · 执行帧"}
				options=append(options,fmt.Sprintf("#%d %s · %s:%d%s",i,SafeText(f.Location.Function,false),SafeText(f.Location.File,false),f.Location.Line,tag))
			}
		}
		return options
	}
	return nil
}
func (m *Model) handleWorkspace(msg tea.KeyMsg) tea.Cmd {
	k:=msg.String()
	if k=="esc" {m.mode="";return nil}
	if k=="ctrl+c" || k=="q" {return tea.Quit}
	if m.mode=="help" {if k=="enter" || k=="?" {m.mode=""};return nil}
	if m.mode=="jump" || m.mode=="display-range" {
		return m.handleNumeric(k)
	}
	options:=m.menuOptions()
	switch k {
	case "up":m.menuCursor=max(0,m.menuCursor-1)
	case "down":m.menuCursor=min(max(0,len(options)-1),m.menuCursor+1)
	case "home":m.menuCursor=0
	case "end":m.menuCursor=max(0,len(options)-1)
	case "enter":
		if len(options)==0 {m.mode="";return nil}
		switch m.mode {
		case "filter":m.setFilter(m.menuCursor)
		case "stack":
			m.replay.Frame=m.menuCursor;m.sourceLine=0;m.sourceColumn=0;m.varCursor=0;m.resetScroll();m.mode=""
		case "object":
			i:=m.menuCursor;m.mode=""
			switch i {
			case 0:m.openInspector("detail")
			case 1:m.openInspector("history")
			case 2:m.setFilter(2)
			case 3:if r,ok:=m.selectedVariable();ok {m.toggleFull(r)}
			case 4:
				if row,ok:=m.selectedVariable();ok && row.Raw.Kind==observer.Array {
					m.openDisplayRange(row)
				}
			case 5:m.openInspector("range")
			case 6:m.toggleTree()
			}
			m.targetPosition=m.replay.Position
		}
	}
	return nil
}
func (m *Model) openDisplayRange(row VariableRow) {
	m.target=row
	m.mode="display-range"
	m.rangeField=0
	b:=m.displayFocus(row).Rows
	if !b.Valid {
		for _,child:=range row.Raw.Children {if child.Index!=nil {b.Add(*child.Index)}}
	}
	m.rangeInput=[2]string{fmt.Sprint(b.Lo),fmt.Sprint(b.Hi)}
	m.notice=""
}
func (m *Model) handleNumeric(k string) tea.Cmd {
	if m.mode=="jump" {
		switch k {
		case "enter":
			n,err:=strconv.Atoi(m.jumpInput)
			if err!=nil || n<1 || n>len(m.replay.Run.Snapshots) {m.notice="请输入有效快照编号。";return nil}
			m.seek(n-1);m.mode="";m.notice=""
		case "backspace":if len(m.jumpInput)>0 {m.jumpInput=m.jumpInput[:len(m.jumpInput)-1]}
		case "ctrl+u":m.jumpInput=""
		default:if isDigits(k) && len(m.jumpInput)+len(k)<=9 {m.jumpInput+=k}
		}
		return nil
	}
	switch k {
	case "tab","shift+tab","up","down":m.rangeField=1-m.rangeField
	case "ctrl+u":m.rangeInput[m.rangeField]=""
	case "backspace":
		s:=m.rangeInput[m.rangeField];if len(s)>0 {m.rangeInput[m.rangeField]=s[:len(s)-1]}
	case "enter":
		lo,e1:=strconv.Atoi(m.rangeInput[0]);hi,e2:=strconv.Atoi(m.rangeInput[1])
		b:=Bounds{}
		for _,child:=range m.target.Raw.Children {if child.Index!=nil {b.Add(*child.Index)}}
		if e1!=nil || e2!=nil || !b.Valid || lo>hi || lo<b.Lo || hi>b.Hi {m.notice="请输入已采集下标边界内的闭区间。";return nil}
		next:=map[string]FocusRange{}
		for k,v:=range m.manualRanges {next[k]=v}
		next[m.target.Key]=FocusRange{Rows:Bounds{Lo:lo,Hi:hi,Valid:true}}
		m.manualRanges=next
		full:=map[string]bool{}
		for k,v:=range m.fullArrays {full[k]=v}
		delete(full,m.target.Key);m.fullArrays=full
		m.mode="";m.notice=""
	default:
		if isDigits(k) && len(m.rangeInput[m.rangeField])+len(k)<=18 {m.rangeInput[m.rangeField]+=k}
	}
	return nil
}
func isDigits(s string) bool {
	if s=="" {return false}
	for _,c:=range s {if c<'0' || c>'9' {return false}}
	return true
}

func (m Model) workspaceOverlay(w,h int) string {
	titles:=map[string]string{"filter":"导航模式","stack":"选择调用栈帧","jump":"跳转快照","help":"操作帮助","object":"对象操作","display-range":"展示范围"}
	lines:=[]string{accent.Render(clip("C Observer / "+titles[m.mode],w)),""}
	switch m.mode {
	case "help":
		for _,line:=range []string{
			"←/→ 回放    Home/End 首尾    J 指定快照",
			"↑/↓ 选择    Tab/Shift+Tab 切换区域",
			"Enter 详情或确认    Esc 返回    Q 退出",
			"M 最大化/恢复    F 导航筛选    H 历史",
			"S 调用栈    O 对象菜单    T 树状展开",
			"D 数据表    V 全部/自动范围    A 采集范围",
			"[/] 源码横向移动 / 数据表翻列",
			"R 重跑    E 修改独立启动配置",
			"值变化来自相邻已采集快照，不等于完整赋值日志。",
		} {lines=append(lines,clip(line,w))}
	case "jump":
		lines=append(lines,clip(fmt.Sprintf("快照编号 (1..%d): %s▏",len(m.replay.Run.Snapshots),m.jumpInput),w))
	case "display-range":
		lines=append(lines,clip("只调整已采集内容的展示，不重新运行。",w))
		for i,name:=range []string{"起始下标","结束下标"} {
			mark:=" ";if m.rangeField==i {mark=">"}
			lines=append(lines,clip(mark+" "+name+"  "+m.rangeInput[i],w))
		}
	default:
		options:=m.menuOptions()
		n:=max(1,h-5)
		start:=max(0,min(m.menuCursor-n+1,max(0,len(options)-n)))
		for i:=start;i<min(len(options),start+n);i++ {
			label:=clip("  "+options[i],w)
			if i==m.menuCursor {label=selected.Render(clip("> "+options[i],w))}
			lines=append(lines,label)
		}
		if len(options)==0 {lines=append(lines,"无已记录项目")}
	}
	if m.notice!="" {lines=append(lines,warning.Render(clip(m.notice,w)))}
	lines=append(lines,muted.Render(clip("↑/↓ 或 Tab 选择  Enter 确认  Esc 返回",w)))
	return fixed(lines,w,h)
}
