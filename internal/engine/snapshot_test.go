package engine

import (
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
)

func TestArrayRangeSelection(t *testing.T) {
	outer := &observer.SourceLocation{File: "main.c", Line: 2}
	inner := &observer.SourceLocation{File: "main.c", Line: 9}
	const path = "records[0].scores"
	pages := []observer.ArrayRange{
		{Path: path, Start: 5, Count: 1},
		{Path: path, Declaration: inner, Start: 12, Count: 1},
		{Path: path, Declaration: inner, Start: 14, Count: 1},
		{Path: path, Start: 6, Count: 1},
	}
	for _, tc := range []struct {
		name        string
		declaration *observer.SourceLocation
		start       int
	}{
		{"matching declaration wins over both defaults", inner, 14},
		{"shadowed declaration gets latest default", outer, 6},
		{"missing declaration uses only default", nil, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := arrayRange(pages, path, tc.declaration); got == nil || got.Start != tc.start {
				t.Fatalf("selected %+v, want start %d", got, tc.start)
			}
		})
	}
}


func TestPointerChildrenAllowed(t *testing.T) {
	for _, tc := range []struct{typ, value string; want bool}{
		{"Team *", "0x1234", true}, {"const struct Team *", "0x1234", true},
		{"void *", "0x1234", false}, {"Team **", "0x1234", false},
		{"int (*)(int)", "0x1234", false}, {"Team *", "0x0000", false},
		{"Team *", "<unavailable>", false},
	} {
		if got:=pointerChildrenAllowed(tc.typ,tc.value);got!=tc.want{t.Errorf("%s %s: %v",tc.typ,tc.value,got)}
	}
}
func TestUniqueGlobalFallback(t *testing.T) {
	a:=declarationKey{"teams","main.c",4}
	i:=sourceIndex{globals:map[declarationKey]observer.VariableScope{a:observer.GlobalScope}}
	if k,s:=i.uniqueGlobal("teams");k!=a||s!=observer.GlobalScope{t.Fatal(k,s)}
	i.globals[declarationKey{"teams","main.c",9}]=observer.FunctionStaticScope
	if _,s:=i.uniqueGlobal("teams");s!=""{t.Fatal("ambiguous global matched")}
}
func TestPointeeNotMarkedAsPointerIdentity(t *testing.T){
	v:=observer.Variable{Kind:observer.Pointer,Children:[]observer.Variable{{Name:"score"}}}
	markObject(&v,"root")
	if !v.IdentityReliable||v.Children[0].IdentityReliable{t.Fatal(v)}
}
