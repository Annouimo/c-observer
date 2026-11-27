// Package app owns a single observation session independently of any GUI.
package app

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "sync"

 "github.com/Routilet/c-observer/internal/observer"
 "github.com/Routilet/c-observer/internal/record"
)

const MaxSample = 4 << 20
var ErrBusy = errors.New("已有观察任务正在运行")
var ErrClosing = errors.New("应用正在关闭")

type Controller struct {
 mu sync.Mutex
 engine observer.Engine
 version string
 cancel context.CancelFunc
 done chan struct{}
 closing bool
 last *record.ObservationRecord
 config *observer.LaunchConfig
}
func New(engine observer.Engine,version string)*Controller{
 return &Controller{engine:engine,version:version}
}
func Standalone(path,stdin string,expected *string,base observer.LaunchConfig)(observer.LaunchConfig,error){
 if strings.TrimSpace(path)==""||!strings.EqualFold(filepath.Ext(path),".c"){return base,fmt.Errorf("请选择 C 源文件（.c）")}
 p,err:=filepath.Abs(path);if err!=nil{return base,err}
 info,err:=os.Stat(p);if err!=nil||!info.Mode().IsRegular(){return base,fmt.Errorf("无法读取源文件：%s",p)}
 if len(stdin)>MaxSample||expected!=nil&&len(*expected)>MaxSample{return base,fmt.Errorf("样例超过 4 MiB")}
 // Only keep the independent invocation's explicit tool overrides.
 cfg:=observer.LaunchConfig{SourcePath:p,WorkDir:filepath.Dir(p),Compiler:base.Compiler,Debugger:base.Debugger,Backend:base.Backend,Stdin:[]byte(stdin)}
 if expected!=nil{v:=[]byte(*expected);cfg.ExpectedStdout=&v}
 return cfg,nil
}
func (c *Controller) Run(cfg observer.LaunchConfig)(record.ObservationRecord,error){
 c.mu.Lock()
 if c.closing{c.mu.Unlock();return record.ObservationRecord{},ErrClosing}
 if c.done!=nil{c.mu.Unlock();return record.ObservationRecord{},ErrBusy}
 ctx,cancel:=context.WithCancel(context.Background())
 done:=make(chan struct{})
 c.cancel,c.done=cancel,done
 c.mu.Unlock()
 defer func(){cancel();c.mu.Lock();c.cancel=nil;c.done=nil;close(done);c.mu.Unlock()}()
 result:=c.engine.Run(ctx,cfg)
 r:=record.Capture(c.version,cfg,result)
 if err:=r.Validate();err!=nil{return record.ObservationRecord{},err}
 c.mu.Lock();c.last=&r;c.config=&cfg;c.mu.Unlock()
 return r,nil
}
func (c *Controller) Rerun()(record.ObservationRecord,error){
 c.mu.Lock();cfg:=c.config;c.mu.Unlock()
 if cfg==nil{return record.ObservationRecord{},errors.New("当前记录不能重跑")}
 return c.Run(*cfg)
}
func (c *Controller) Cancel(){
 c.mu.Lock();defer c.mu.Unlock()
 if c.cancel!=nil{c.cancel()}
}
// Close waits for the engine's bounded cleanup before returning. No new
// observations may start after shutdown, even if a frontend call races it.
func (c *Controller) Close(){
 c.mu.Lock();c.closing=true
 if c.cancel!=nil{c.cancel()}
 done:=c.done;c.mu.Unlock()
 if done!=nil{<-done}
}
func (c *Controller) Export(dir string)(string,error){
 c.mu.Lock();defer c.mu.Unlock()
 if c.done!=nil{return "",ErrBusy}
 if c.last==nil{return "",errors.New("没有现场观察记录")}
 return record.SaveRecord(dir,*c.last)
}
func Encode(r record.ObservationRecord)(string,error){
 b,err:=json.Marshal(r);return string(b),err
}
