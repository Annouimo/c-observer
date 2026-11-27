package app

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "testing"
 "time"

 "github.com/Routilet/c-observer/internal/observer"
 "github.com/Routilet/c-observer/internal/record"
 "github.com/Routilet/c-observer/internal/backend"
)

type fakeEngine func(context.Context,observer.LaunchConfig)observer.RunResult
func(f fakeEngine)Run(ctx context.Context,c observer.LaunchConfig)observer.RunResult{return f(ctx,c)}
func TestCancelAndClose(t *testing.T){
 entered:=make(chan struct{})
 cleaned:=make(chan struct{})
 c:=New(fakeEngine(func(ctx context.Context,cfg observer.LaunchConfig)observer.RunResult{
  close(entered);<-ctx.Done();close(cleaned)
  return observer.RunResult{Reason:observer.Cancelled,Session:observer.Session{Backend:"gdb-mi"},Snapshots:[]observer.Snapshot{{ID:0}}}
 }),"dev")
 done:=make(chan record.ObservationRecord,1)
 go func(){r,e:=c.Run(observer.LaunchConfig{});if e!=nil{t.Error(e)};done<-r}()
 select{case <-entered:case <-time.After(time.Second):t.Fatal("engine did not start")}
 if _,e:=c.Run(observer.LaunchConfig{});!errors.Is(e,ErrBusy){t.Fatalf("busy: %v",e)}
 c.Cancel();c.Close()
 select{case <-cleaned:default:t.Fatal("cleanup not finished")}
 r:=<-done;if r.Run.Reason!=observer.Cancelled||len(r.Run.Snapshots)!=1{t.Fatal("partial record lost")}
 if _,e:=c.Run(observer.LaunchConfig{});!errors.Is(e,ErrClosing){t.Fatalf("closed: %v",e)}
}
func TestStandaloneRerunExport(t *testing.T){
 dir:=t.TempDir();p:=filepath.Join(dir,"中文 source.c")
 if e:=os.WriteFile(p,[]byte("int main(void){return 0;}"),0600);e!=nil{t.Fatal(e)}
 empty:=""
 cfg,e:=Standalone(p,"1\n",&empty,observer.LaunchConfig{SourceMap:map[string]string{"old":"old"}})
 if e!=nil||cfg.ExpectedStdout==nil||len(*cfg.ExpectedStdout)!=0||cfg.SourceMap!=nil||cfg.WorkDir!=dir{t.Fatalf("config: %+v %v",cfg,e)}
 var calls int
 c:=New(fakeEngine(func(_ context.Context,x observer.LaunchConfig)observer.RunResult{
  calls++;return observer.RunResult{Reason:observer.Exited,Session:observer.Session{Backend:"gdb-mi"},Output:observer.OutputState{Stdout:[]byte{byte(calls)}}}
 }),"dev")
 if _,e=c.Run(cfg);e!=nil{t.Fatal(e)}
 r,e:=c.Rerun();if e!=nil||calls!=2||r.Run.Output.Stdout[0]!=2{t.Fatal("rerun",e)}
 path,e:=c.Export(dir);if e!=nil{t.Fatal(e)}
 got,e:=record.LoadRecord(path);if e!=nil||got.ExpectedStdout==nil||got.Run.Output.Stdout[0]!=2{t.Fatal("export",e)}
}
func TestRealApplication(t *testing.T){
 if os.Getenv("C_OBSERVER_REAL_APP")!="1"{t.Skip("opt-in native debugger test")}
 p:=filepath.Join(t.TempDir(),"main.c")
 if e:=os.WriteFile(p,[]byte("#include <stdio.h>\nint main(void) {\n int x=0;\n scanf(\"%d\", &x);\n x+=2;\n printf(\"%d\\n\",x);\n return 0;\n}\n"),0600);e!=nil{t.Fatal(e)}
 expected:="7\n";cfg,e:=Standalone(p,"5\n",&expected,observer.LaunchConfig{})
 if e!=nil{t.Fatal(e)}
 c:=New(backend.SelectedEngine{},"dev");defer c.Close()
 r,e:=c.Run(cfg);if e!=nil||r.Run.Reason!=observer.Exited||len(r.Run.Snapshots)==0||stringsTrimCR(string(r.Run.Output.Stdout))!=expected{t.Fatalf("real run: %v %+v",e,r.Run)}
}
func stringsTrimCR(s string)string{
 out:=make([]byte,0,len(s));for i:=range len(s){if s[i]!='\r'{out=append(out,s[i])}};return string(out)
}
