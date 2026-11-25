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
	flag.Parse()
	if err:=run(*path);err!=nil {fmt.Fprintln(os.Stderr,err);os.Exit(1)}
}
func run(path string) error {
	f,err:=os.Open(path);if err!=nil{return err}
	defer f.Close()
	fixture,err:=tui.DecodeFixture(f);if err!=nil{return err}
	_,err=tea.NewProgram(tui.NewPreview(fixture),tea.WithAltScreen()).Run()
	return err
}
