// Command c-observer-preview is a development-only preview, including an
// explicit live mode for integration review. Production wiring is separate.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Annouimo/c-observer/internal/engine"
	"github.com/Annouimo/c-observer/internal/observer"
	"github.com/Annouimo/c-observer/internal/tui"
)

func main() {
	path:=flag.String("fixture","testdata/snapshots/wrong-index.json","synthetic fixture path")
	setup:=flag.Bool("setup",false,"preview standalone configuration form")
	live:=flag.Bool("live",false,"observe a real local C program with the published LLDB engine")
	source:=flag.String("source","","C source file for live mode")
	input:=flag.String("input","","sample stdin file for live mode")
	expected:=flag.String("expected","","optional expected stdout file for live mode")
	compiler:=flag.String("compiler","","compiler path override")
	debugger:=flag.String("debugger","","debug adapter path override")
	dump:=flag.Bool("dump",false,"print synthetic preview without entering an alternate terminal")
	width:=flag.Int("width",100,"dump width in terminal cells")
	height:=flag.Int("height",30,"dump height in terminal cells")
	step:=flag.Int("step",1,"dump snapshot number (1-based; zero for final result)")
	flag.Parse()
	var model tui.Model
	var err error
	if *live {
		cfg:=observer.LaunchConfig{Compiler:*compiler,Debugger:*debugger}
		if *source!="" {
			cfg.SourcePath,err=filepath.Abs(*source)
			if err==nil {cfg.WorkDir=filepath.Dir(cfg.SourcePath)}
		}
		if err==nil && *input!="" {cfg.Stdin,err=os.ReadFile(*input)}
		if err==nil && *expected!="" {var data []byte;data,err=os.ReadFile(*expected);cfg.ExpectedStdout=&data}
		if err==nil {
			if *setup || *source=="" {model=tui.NewSetup(engine.Engine{},cfg)} else {model=tui.NewLive(engine.Engine{},cfg)}
		}
	} else {
		if *source!="" || *input!="" || *expected!="" || *compiler!="" || *debugger!="" {
			err=fmt.Errorf("source, input, expected and toolchain flags require explicit -live")
		} else {model,err=preview(*path,*setup)}
	}
	if err==nil && *dump {
		if *live {err=fmt.Errorf("-dump only supports synthetic preview")} else {
			var rendered string
			rendered,err=model.PreviewText(*width,*height,*step)
			if err==nil {fmt.Println(rendered)}
		}
	} else if err==nil {_,err=tea.NewProgram(model,tea.WithAltScreen()).Run()}
	if err!=nil {fmt.Fprintln(os.Stderr,err);os.Exit(1)}
}
func preview(path string,setup bool) (tui.Model,error) {
	f,err:=os.Open(path);if err!=nil{return tui.Model{},err}
	defer f.Close()
	fixture,err:=tui.DecodeFixture(f);if err!=nil{return tui.Model{},err}
	if setup {return tui.NewSetupPreview(fixture),nil}
	return tui.NewPreview(fixture),nil
}
