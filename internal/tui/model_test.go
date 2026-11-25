package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)
func TestNavigation(t *testing.T) {
	m:=NewPreview(fixture(t,"wrong-index"))
	next,_:=m.Update(tea.KeyMsg{Type:tea.KeyRight})
	m=next.(Model)
	if m.replay.Position!=1 {t.Fatal("next")}
	next,_=m.Update(tea.KeyMsg{Type:tea.KeyEnd})
	m=next.(Model)
	if !m.replay.End() || !strings.Contains(m.View(),"END") {t.Fatal("end")}
	next,_=m.Update(tea.KeyMsg{Type:tea.KeyHome})
	m=next.(Model)
	if m.replay.Position!=0 {t.Fatal("home")}
}
func TestRenderSizes(t *testing.T) {
	for _,name:=range []string{"wrong-index","cancelled","compile-failed","limited"} {
		m:=NewPreview(fixture(t,name))
		for _,size:=range [][2]int{{100,30},{86,14},{50,16},{30,10}} {
			m.width,m.height=size[0],size[1]
			for p:=0;p<=len(m.replay.Run.Snapshots);p++ {
				m.replay.Seek(p)
				for focus:=0;focus<3;focus++ {m.focus=focus;if m.View()=="" {t.Fatal("empty view")}}
			}
		}
	}
}
func TestLexer(t *testing.T) {
	lines:=lexC("int x = 1; /* comment\ncontinued */ char *s = \"//\";")
	if len(lines)!=2 || lines[0][0].kind!=keyword || lines[1][0].kind!=comment {t.Fatal("lexer state")}
	found:=false
	for _,tok:=range lines[1] {if tok.kind==literal && tok.text=="\"//\"" {found=true}}
	if !found {t.Fatal("string parsed as comment")}
}
