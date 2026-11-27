package main

import (
 "embed"
 "encoding/json"
 "flag"
 "fmt"
 "os"
 "path/filepath"
 "runtime"
 "strings"

 "github.com/Routilet/c-observer/internal/app"
 "github.com/Routilet/c-observer/internal/backend"
 "github.com/Routilet/c-observer/internal/observer"
 "github.com/Routilet/c-observer/internal/record"
 "github.com/Routilet/c-observer/internal/session"
 "github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS
var version = "dev"

type startup struct {
 Mode string
 Source string
 Record string
 Error string
}
type Desktop struct {
 native *application.App
 core *app.Controller
 initial startup
 base observer.LaunchConfig
 sessionConfig *observer.LaunchConfig
}
func(d *Desktop) Initial() (string,error){b,e:=json.Marshal(d.initial);return string(b),e}
func(d *Desktop) ChooseSource()(string,error){
 return d.native.Dialog.OpenFile().SetTitle("选择 C 源文件").AddFilter("C 源文件","*.c").PromptForSingleSelection()
}
func(d *Desktop) OpenRecord()(string,error){
 path,e:=d.native.Dialog.OpenFile().SetTitle("打开观察记录").AddFilter("观察记录","*.json").PromptForSingleSelection()
 if e!=nil||path==""{return "",e}
 r,e:=record.LoadRecord(path);if e!=nil{return "",e};return app.Encode(r)
}
func(d *Desktop) Run(source,stdin string,withExpected bool,expected string)(string,error){
 var want *string;if withExpected{want=&expected}
 cfg,e:=app.Standalone(source,stdin,want,d.base);if e!=nil{return "",e}
 r,e:=d.core.Run(cfg);if e!=nil{return "",e};return app.Encode(r)
}
func(d *Desktop) RunSession()(string,error){
 if d.sessionConfig==nil{return "",fmt.Errorf("没有可运行的会话")}
 r,e:=d.core.Run(*d.sessionConfig);if e!=nil{return "",e};return app.Encode(r)
}
func(d *Desktop) Rerun()(string,error){
 r,e:=d.core.Rerun();if e!=nil{return "",e};return app.Encode(r)
}
func(d *Desktop) Cancel(){d.core.Cancel()}
func(d *Desktop) Export()(string,error){
 dir,e:=os.UserHomeDir();if e!=nil{return "",e}
 return d.core.Export(dir)
}

func run() error {
 flags:=flag.NewFlagSet("c-observer",flag.ContinueOnError)
 ver:=flags.Bool("version",false,"print version")
 caps:=flags.Bool("capabilities",false,"print protocol capabilities")
 sess:=flags.String("session","","launch session")
 replay:=flags.String("replay","","open a saved record")
 be:=flags.String("backend","auto","auto, gdb or lldb")
 cc:=flags.String("compiler","","compiler path or name")
 dbg:=flags.String("debugger","","debugger path or name")
 if e:=flags.Parse(os.Args[1:]);e!=nil{if e==flag.ErrHelp{return nil};return e}
 if *ver{fmt.Println(version);return nil}
 if *caps{return json.NewEncoder(os.Stdout).Encode(map[string]any{"version":version,"sessionSchemaVersions":[]int{session.Version},"recordSchemaVersions":[]int{record.RecordVersion},"platform":runtime.GOOS,"arch":runtime.GOARCH,"backends":backend.SupportedBackends(runtime.GOOS),"sessionFeatures":[]string{"backend","comparison"}})}
 d:=&Desktop{core:app.New(backend.SelectedEngine{},version),initial:startup{Mode:"setup"},base:observer.LaunchConfig{Backend:*be,Compiler:*cc,Debugger:*dbg}}
 defer d.core.Close()
 if flags.NArg()>1||*sess!=""&&*replay!=""||(*sess!=""||*replay!="")&&(flags.NArg()!=0||*be!="auto"||*cc!=""||*dbg!=""){return fmt.Errorf("不兼容的启动参数")}
 if *sess!=""{
  cfg,e:=session.Load(*sess);if e!=nil{d.initial.Error=e.Error()}else{d.sessionConfig=&cfg;d.initial.Mode="session";d.initial.Source=cfg.SourcePath}
 }else if *replay!=""{
  r,e:=record.LoadRecord(*replay);if e!=nil{d.initial.Error=e.Error()}else{d.initial.Record,e=app.Encode(r);if e!=nil{return e};d.initial.Mode="replay"}
 }else if flags.NArg()==1{
  p,e:=filepath.Abs(flags.Arg(0));if e!=nil{return e}
  if !strings.EqualFold(filepath.Ext(p),".c"){return fmt.Errorf("仅支持 C 源文件")}
  d.initial.Source=p
 }
 native:=application.New(application.Options{
  Name:"C Observer Preview",Description:"Offline C execution observer",
  Services:[]application.Service{application.NewService(d)},
  Assets:application.AssetOptions{Handler:application.BundledAssetFileServer(assets)},
  Mac:application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed:true},
  OnShutdown:d.core.Close,
 })
 d.native=native
 native.Window.NewWithOptions(application.WebviewWindowOptions{Title:"C Observer Preview",Width:980,Height:720,URL:"/"})
 return native.Run()
}
func main(){if e:=run();e!=nil{fmt.Fprintln(os.Stderr,"C Observer:",e);os.Exit(1)}}
