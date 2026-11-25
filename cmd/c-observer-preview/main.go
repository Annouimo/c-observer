// Command c-observer-preview is a development-only synthetic replay entrypoint.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Annouimo/c-observer/internal/tui"
)

func main() {
	path:=flag.String("fixture","testdata/snapshots/wrong-index.json","synthetic fixture path")
	setup:=flag.Bool("setup",false,"preview standalone configuration form without executing C")
	flag.Parse()
	if err:=run(*path,*setup);err!=nil {fmt.Fprintln(os.Stderr,err);os.Exit(1)}
}
func run(path string,setup bool) error {
	f,err:=os.Open(path);if err!=nil{return err}
	defer f.Close()
	fixture,err:=tui.DecodeFixture(f);if err!=nil{return err}
	model:=tui.NewPreview(fixture)
	if setup {model=tui.NewSetupPreview(fixture)}
	_,err=tea.NewProgram(model,tea.WithAltScreen()).Run()
	return err
}
