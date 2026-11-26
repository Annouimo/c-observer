package engine

import (
 "path/filepath"
 "testing"
 "github.com/Routilet/c-observer/internal/observer"
)

func TestRelativeLogicalMapFromAbsoluteDebuggerPath(t *testing.T) {
 dir:=t.TempDir()
 original:=filepath.Join(dir,"frozen.c")
 c:=capture{dir:dir, files:map[string]string{original:"int main(void){return 0;}"},config:observer.LaunchConfig{SourceMap:map[string]string{"logical.c":original}},result:&observer.RunResult{}}
 if loc:=c.location(filepath.Join(dir,"logical.c"),1,"main");loc.File!=original {t.Fatalf("unmapped absolute logical path: %+v",loc)}
 if loc:=c.location(filepath.Join(dir,"other","logical.c"),1,"main");loc.File!="" {t.Fatalf("incorrect basename guess: %+v",loc)}
}
