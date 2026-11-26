package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	for name,content := range map[string]string{"observe.c":"int main(void){return 0;}","main.c":"original","sample.in":"1\r\n","sample.out":"","session.json":`{"schemaVersion":1,"sourcePath":"observe.c","workDir":".","visibleSourcePaths":["main.c"],"sourceMap":{"main.c":"main.c"},"stdinFile":"sample.in","expectedStdoutFile":"sample.out"}`} {
		if err := os.WriteFile(filepath.Join(dir,name),[]byte(content),0600); err != nil {t.Fatal(err)}
	}
	cfg,err:=Load(filepath.Join(dir,"session.json"))
	if err!=nil {t.Fatal(err)}
	if cfg.SourceMap["main.c"]!=filepath.Join(dir,"main.c") || string(cfg.Stdin)!="1\r\n" || cfg.ExpectedStdout==nil || len(*cfg.ExpectedStdout)!=0 {t.Fatalf("bad config %+v",cfg)}
	for _,body:=range []string{`{"schemaVersion":2}`,`{"schemaVersion":1,"unexpected":true}`,`{} {}`,`{"schemaVersion":1,"sourcePath":"observe.c","stdinFile":"missing"}`} {
		p:=filepath.Join(dir,"bad.json")
		if err:=os.WriteFile(p,[]byte(body),0600);err!=nil {t.Fatal(err)}
		if _,err:=Load(p);err==nil {t.Fatalf("accepted %s",body)}
	}
}
