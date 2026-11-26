package main
import "testing"
func TestBackendSelection(t *testing.T){
 yes:=func(string)bool{return true}
 no:=func(string)bool{return false}
 for _,c:=range []struct{platform,requested,compiler,debugger,want string;exists func(string)bool}{
 {"windows","auto","","","gdb",no},{"darwin","auto","","","lldb",no},{"linux","auto","","","gdb",yes},{"linux","auto","","","lldb",no},{"linux","auto","clang","","lldb",yes},{"linux","auto","gcc","lldb-dap","lldb",yes},{"linux","gdb","","","gdb",no},
 }{got,err:=chooseBackend(c.platform,c.requested,c.compiler,c.debugger,c.exists);if err!=nil||got!=c.want{t.Fatalf("%+v: %s %v",c,got,err)}}
 if _,err:=chooseBackend("windows","lldb","","",yes);err==nil{t.Fatal("unsupported backend accepted")}
}
