package tui

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

type OutputLine struct {
	Expected, Actual string
	HasExpected, HasActual, Equal bool
}

// Split preserves empty lines and whether the final line ended with LF.
// Compare uses raw bytes, not the escaped display strings.
func outputLines(b []byte) [][]byte {
	if len(b)==0 {return nil}
	out:=bytes.SplitAfter(b,[]byte("\n"))
	if len(out)>0 && len(out[len(out)-1])==0 {out=out[:len(out)-1]}
	return out
}
func visibleLine(b []byte) string {
	var s strings.Builder
	for len(b)>0 {
		r,n:=utf8.DecodeRune(b)
		if r==utf8.RuneError && n==1 {fmt.Fprintf(&s,"\\x%02x",b[0]);b=b[1:];continue}
		switch r {
		case '\n':s.WriteString("↵")
		case '\r':s.WriteString("\\r")
		case '\t':s.WriteString("\\t")
		case ' ':s.WriteString("·")
		default:s.WriteString(SafeText(string(r),false))
		}
		b=b[n:]
	}
	return s.String()
}
func CompareOutput(expected,actual []byte) []OutputLine {
	a,b:=outputLines(expected),outputLines(actual)
	out:=make([]OutputLine,0,max(len(a),len(b)))
	for i:=0;i<max(len(a),len(b));i++ {
		line:=OutputLine{HasExpected:i<len(a),HasActual:i<len(b)}
		if line.HasExpected {line.Expected=visibleLine(a[i])}
		if line.HasActual {line.Actual=visibleLine(b[i])}
		line.Equal=line.HasExpected && line.HasActual && bytes.Equal(a[i],b[i])
		out=append(out,line)
	}
	return out
}
func (m Model) outputComparison(w,h int) string {
	lines:=[]string{head(" 输出对照 · 逐字节比较（非 OJ 判定）",m.focus==2,w)}
	if m.config.ExpectedStdout==nil {
		return fixed(append(lines,"未提供预期输出。"),w,h)
	}
	out,_:=m.replay.Output()
	if !m.replay.End() {
		lines=append(lines,muted.Render(clip("当前已捕获前缀，尚不能判定最终输出。",w)))
	} else if m.replay.Run.Output.StdoutTruncated {
		lines=append(lines,warning.Render(clip("实际输出已截断，无法确认完整结果。",w)))
	} else if m.replay.Run.Reason!="exited" {
		lines=append(lines,warning.Render(clip("程序未正常结束；仅比较已捕获输出。",w)))
	} else {
		msg:="输出有差异"
		if bytes.Equal(*m.config.ExpectedStdout,out) {msg="输出一致"}
		lines=append(lines,muted.Render(msg))
	}
	lines=append(lines,muted.Render(clip("· 空格  ↵ 换行  \\t 制表符  \\r 回车",w)))
	rows:=CompareOutput(*m.config.ExpectedStdout,out)
	if len(rows)==0 {lines=append(lines,"两者均为空。");return fixed(lines,w,h)}
	// Pair lines by position (not an edit-distance alignment). Wrap long
	// values so the entire content remains inspectable.
	avail:=max(1,h-len(lines))
	cw:=max(1,(w-9)/2)
	tableLines:=[]string{muted.Render(pad("行",5)+" "+pad(clip("预期",cw),cw)+" │ "+clip("实际",cw))}
	for i,r:=range rows {
		e,a:=r.Expected,r.Actual
		if !r.HasExpected {e="（缺行）"}
		if !r.HasActual {a="（缺行）"}
		el:=wrapCells(e,cw);al:=wrapCells(a,cw)
		for j:=0;j<max(len(el),len(al));j++ {
			prefix:="";if j==0 {prefix=fmt.Sprint(i+1)}
			l,rr:="",""
			if j<len(el) {l=el[j]}
			if j<len(al) {rr=al[j]}
			if !r.Equal {rr=failure.Render(pad(clip(rr,cw),cw))} else {rr=pad(clip(rr,cw),cw)}
			tableLines=append(tableLines,pad(prefix,5)+" "+pad(clip(l,cw),cw)+" │ "+rr)
		}
	}
	start:=max(0,min(m.consoleScroll,max(0,len(tableLines)-avail)))
	lines=append(lines,tableLines[start:min(len(tableLines),start+avail)]...)
	return fixed(lines,w,h)
}

func wrapCells(text string,w int) []string {
	return strings.Split(runewidth.Wrap(text,max(1,w)),"\n")
}
