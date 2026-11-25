//go:build darwin || linux

package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Annouimo/c-observer/internal/engine"
	"github.com/Annouimo/c-observer/internal/observer"
)

func TestRealEngineReplay(t *testing.T) {
	if os.Getenv("C_OBSERVER_REAL_DEBUGGER")!="1" {t.Skip("real debugger test is opt-in")}
	dir:=t.TempDir()
	src:=filepath.Join(dir,"课程 example.c")
	program:=`#include <stdio.h>
int main(void) {
    int a[100010] = {0};
    int n, sum = 0;
    scanf("%d", &n);
    for (int i=0; i<n; ++i) {
        scanf("%d", &a[i]);
        sum += a[i];
    }
    printf("%d\n", sum);
    return 0;
}
`
	if err:=os.WriteFile(src,[]byte(program),0600);err!=nil {t.Fatal(err)}
	expected:=[]byte("10\n")
	cfg:=observer.LaunchConfig{SourcePath:src,WorkDir:dir,Stdin:[]byte("3\n3 0 7\n"),ExpectedStdout:&expected,Compiler:os.Getenv("C_OBSERVER_TEST_CLANG"),Debugger:os.Getenv("C_OBSERVER_TEST_LLDB")}
	result:= (engine.Engine{}).Run(context.Background(),cfg)
	if result.Reason!=observer.Exited || string(result.Output.Stdout)!="10\n" || len(result.Snapshots)<5 || result.CleanupError!="" {
		t.Fatalf("real run: reason=%s message=%s diagnostics=%+v stdout=%q count=%d cleanup=%s",result.Reason,result.Message,result.Diagnostics,result.Output.Stdout,len(result.Snapshots),result.CleanupError)
	}
	m:=NewLive(engine.Engine{},cfg)
	next,_:=m.Update(runDone{result});m=next.(Model)
	if m.err!="" || m.replay.Index==nil || m.demo {t.Fatal("real result did not enter replay")}
	seenArray:=false
	for pos:=range result.Snapshots {
		m.seek(pos)
		for _,r:=range m.replay.Rows(nil) {
			if r.Name=="a" && r.Raw.Kind==observer.Array && len(r.Raw.Children)>0 {
				seenArray=true
				if !r.Reliable && m.displayFocus(r).Automatic {t.Fatal("unreliable real identity cropped")}
				table:=m.dataTable(r,m.replay.Rows(nil))
				if len(table.Rows)==0 {t.Fatal("real array missing")}
			}
		}
		for _,line:=range strings.Split(m.View(),"\n") {if lipgloss.Width(line)>m.width {t.Fatal("render overflow")}}
	}
	if !seenArray {t.Fatal("real array not captured")}
	next,_=m.Update(tea.KeyMsg{Type:tea.KeyEnd});m=next.(Model)
	if !m.replay.End() || !strings.Contains(m.View(),"输出一致") {t.Fatal("final result missing")}
	t.Logf("real LLDB run replayed %d snapshots",len(result.Snapshots))
}
