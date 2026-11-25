package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

type token struct { text string; kind int }
const (plain=iota; keyword; literal; comment; directive)
var keywords = func() map[string]bool {
	m:=map[string]bool{}
	for _,k:=range strings.Fields("auto break case char const continue default do double else enum extern float for goto if inline int long register restrict return short signed sizeof static struct switch typedef union unsigned void volatile while _Bool _Complex _Imaginary bool true false NULL") {m[k]=true}
	return m
}()
func ident(r rune) bool {return r=='_' || unicode.IsLetter(r) || unicode.IsDigit(r)}
// lexC is a display lexer, not a parser. It maintains multiline comment state.
func lexC(source string) [][]token {
	lines:=strings.Split(source,"\n")
	out:=make([][]token,len(lines))
	inComment:=false
	for i,line:=range lines {
		rs:=[]rune(line)
		add:=func(a,b,k int){out[i]=append(out[i],token{string(rs[a:b]),k})}
		for j:=0;j<len(rs); {
			start:=j
			if inComment || (j+1<len(rs) && rs[j]=='/' && rs[j+1]=='*') {
				if !inComment {j+=2}
				inComment=true
				for j<len(rs) {
					if j+1<len(rs) && rs[j]=='*' && rs[j+1]=='/' {j+=2;inComment=false;break}
					j++
				}
				add(start,j,comment);continue
			}
			if j+1<len(rs) && rs[j]=='/' && rs[j+1]=='/' {add(j,len(rs),comment);break}
			if rs[j]=='"' || rs[j]=='\'' {
				q:=rs[j];j++
				for j<len(rs) {
					if rs[j]=='\\' && j+1<len(rs) {j+=2;continue}
					c:=rs[j];j++;if c==q {break}
				}
				add(start,j,literal);continue
			}
			if rs[j]=='#' && strings.TrimSpace(string(rs[:j]))=="" {
				j++;for j<len(rs) && (ident(rs[j]) || unicode.IsSpace(rs[j])) {j++}
				add(start,j,directive);continue
			}
			if unicode.IsDigit(rs[j]) {
				j++;for j<len(rs) && (ident(rs[j]) || rs[j]=='.') {j++}
				add(start,j,literal);continue
			}
			if ident(rs[j]) {
				j++;for j<len(rs) && ident(rs[j]) {j++}
				k:=plain;if keywords[string(rs[start:j])] {k=keyword}
				add(start,j,k);continue
			}
			j++;add(start,j,plain)
		}
	}
	return out
}
var tokenStyles=map[int]lipgloss.Style{
	keyword: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light:"90",Dark:"141"}),
	literal: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light:"28",Dark:"114"}),
	comment: muted,
	directive: accent,
}
func renderTokens(tokens []token,width int) string {
	var b strings.Builder
	for _,t:=range tokens {
		if width<=0 {break}
		part:=clip(t.text,width)
		if s,ok:=tokenStyles[t.kind];ok {b.WriteString(s.Render(part))} else {b.WriteString(part)}
		width-=runewidth.StringWidth(part)
		if part!=t.text {break}
	}
	return b.String()
}
