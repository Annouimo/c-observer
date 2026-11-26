package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Routilet/c-observer/internal/observer"
	"github.com/Routilet/c-observer/internal/session"
	"github.com/Routilet/c-observer/internal/tui"
)

var version = "dev"

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("c-observer",flag.ContinueOnError)
	flags.SetOutput(out)
	versionOnly := flags.Bool("version",false,"print version")
	capabilities := flags.Bool("capabilities",false,"print machine-readable protocol capabilities")
	sessionPath := flags.String("session","","launch from a versioned session JSON file")
	backend := flags.String("backend","auto","auto, gdb or lldb")
	compiler := flags.String("compiler","","compiler path or name")
	debugger := flags.String("debugger","","debugger path or name")
	flags.Usage = func() {
		fmt.Fprintln(out,"Usage: c-observer [source.c] | c-observer --session session.json")
		flags.PrintDefaults()
	}
	if err:=flags.Parse(args);err!=nil {
		if err==flag.ErrHelp {return nil}
		return err
	}
	if *versionOnly {fmt.Fprintln(out,version);return nil}
	if *capabilities {
		backends:=supportedBackends(runtime.GOOS)
		return json.NewEncoder(out).Encode(map[string]any{"version":version,"sessionSchemaVersions":[]int{session.Version},"platform":runtime.GOOS,"arch":runtime.GOARCH,"backends":backends})
	}
	if len(supportedBackends(runtime.GOOS))==0 {return fmt.Errorf("unsupported platform: %s",runtime.GOOS)}
	if *backend!="auto" && *backend!="gdb" && *backend!="lldb" {return fmt.Errorf("invalid backend: %s",*backend)}
	var model tui.Model
	if *sessionPath!="" {
		if flags.NArg()!=0 || *compiler!="" || *debugger!="" || *backend!="auto" {return fmt.Errorf("--session cannot be combined with source or tool overrides")}
		cfg,err:=session.Load(*sessionPath)
		if err!=nil {return err}
		if _,err:=chooseBackend(runtime.GOOS,cfg.Backend,cfg.Compiler,cfg.Debugger,hasTool);err!=nil{return err}
		model=tui.NewLive(selectedEngine{},cfg)
	} else {
		if flags.NArg()>1 {return fmt.Errorf("expected one C source path")}
		cfg:=observer.LaunchConfig{Backend:*backend,Compiler:*compiler,Debugger:*debugger}
		if _,err:=chooseBackend(runtime.GOOS,cfg.Backend,cfg.Compiler,cfg.Debugger,hasTool);err!=nil{return err}
		if flags.NArg()==1 {
			p,err:=filepath.Abs(flags.Arg(0))
			if err!=nil {return err}
			if strings.ToLower(filepath.Ext(p))!=".c" {return fmt.Errorf("only C source files are supported")}
			cfg.SourcePath,cfg.WorkDir=p,filepath.Dir(p)
		}
		model=tui.NewSetup(selectedEngine{},cfg)
	}
	_,err:=tea.NewProgram(model,tea.WithAltScreen()).Run()
	return err
}

func main() {
	if err:=run(os.Args[1:],os.Stdout);err!=nil {
		fmt.Fprintln(os.Stderr,"C Observer:",err)
		os.Exit(1)
	}
}
