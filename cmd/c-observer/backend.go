package main

import (
 "context"
 "fmt"
 "os/exec"
 "path/filepath"
 "runtime"
 "strings"
 "github.com/Routilet/c-observer/internal/engine"
 "github.com/Routilet/c-observer/internal/observer"
)

func supportedBackends(platform string) []string {
 switch platform {
 case "windows": return []string{"gdb-mi"}
 case "darwin": return []string{"lldb-dap"}
 case "linux": return []string{"gdb-mi","lldb-dap"}
 default: return []string{}
 }
}
func hasTool(name string) bool { _,err:=exec.LookPath(name);return err==nil }
func chooseBackend(platform, requested, compiler, debugger string, exists func(string)bool) (string,error) {
 if requested==""||requested=="auto" {
  switch platform {
  case "windows": requested="gdb"
  case "darwin": requested="lldb"
  case "linux":
   d:=strings.ToLower(filepath.Base(debugger))
   c:=strings.ToLower(filepath.Base(compiler))
   switch {
   case strings.Contains(d,"lldb"): requested="lldb"
   case strings.Contains(d,"gdb"): requested="gdb"
   case strings.Contains(c,"clang"): requested="lldb"
   case strings.Contains(c,"gcc"): requested="gdb"
   case exists("gcc")&&exists("gdb"): requested="gdb"
   default: requested="lldb"
   }
  }
 }
 if (requested=="gdb"&&(platform=="windows"||platform=="linux"))||(requested=="lldb"&&(platform=="darwin"||platform=="linux")) {return requested,nil}
 return "",fmt.Errorf("backend %q is not supported on %s",requested,platform)
}
type selectedEngine struct{}
func (selectedEngine) Run(ctx context.Context,cfg observer.LaunchConfig) observer.RunResult {
 backend,err:=chooseBackend(runtime.GOOS,cfg.Backend,cfg.Compiler,cfg.Debugger,hasTool)
 if err!=nil{return observer.RunResult{Reason:observer.InvalidConfig,Message:err.Error()}}
 if backend=="gdb" {
  if runtime.GOOS=="windows"&&cfg.Debugger=="" {
   compiler:=cfg.Compiler
   if compiler=="" {compiler="gcc"}
   if p,e:=exec.LookPath(compiler);e==nil {
    candidate:=filepath.Join(filepath.Dir(p),"gdb.exe")
    if hasTool(candidate){cfg.Debugger=candidate}
   }
  }
  return (engine.GDBEngine{}).Run(ctx,cfg)
 }
 return (engine.Engine{}).Run(ctx,cfg)
}
