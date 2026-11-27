package engine

import (
	"context"
	"os"
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
)

func loadLoopIndex(t *testing.T, name string) (sourceIndex, mainIdentity) {
	t.Helper()
	text, err := os.ReadFile("../../testdata/programs/" + name)
	if err != nil {
		t.Fatal(err)
	}
	index := inspectSource(context.Background(), "clang", "../../testdata/programs", name, string(text), nil, nil)
	identity := mainIdentity{
		source: index.mainSource, lines: index.mainLines,
		loops: index.mainLoops, decls: index.mainDecls, hasGoto: index.mainHasGoto,
	}
	return index, identity
}

func TestLoopDeclarationIndex(t *testing.T) {
	index, identity := loadLoopIndex(t, "loop_identity.c")
	if index.mainSource == "" || len(index.mainLoops) == 0 {
		t.Fatalf("missing loop index: source=%q loops=%d", index.mainSource, len(index.mainLoops))
	}
	counts := map[string]int{}
	for _, loop := range index.mainLoops {
		counts[loop.name]++
	}
	want := map[string]int{"i": 3, "outer": 1, "inner": 1, "shadow": 2, "a": 1, "b": 1, "q": 1, "z": 1, "c": 1}
	for name, n := range want {
		if counts[name] != n {
			t.Errorf("loop %q: got %d declarations, want %d", name, counts[name], n)
		}
	}
	for _, loop := range index.mainLoops {
		top := loop.name == "i" || loop.name == "outer" || loop.name == "a" || loop.name == "b" || loop.name == "q" || loop.name == "c" ||
			(loop.name == "shadow" && loop.declLine == 31)
		if loop.topLevel != top {
			t.Errorf("loop %q at line %d: topLevel=%v, want %v", loop.name, loop.loopLine, loop.topLevel, top)
		}
	}
	if index.mainHasGoto {
		t.Fatal("unexpected goto in loop_identity.c")
	}
	for _, tc := range []struct {
		name string
		line int
		want bool
	}{
		{"i", 16, true},
		{"i", 19, true},
		{"i", 22, true},
		{"outer", 26, true},
		{"inner", 27, false},
		{"shadow", 32, false},
		{"q", 55, true},
		{"z", 62, false},
		{"a", 38, true},
	} {
		got := identity.loopAt(tc.name, tc.line) != nil
		if got != tc.want {
			t.Errorf("loopAt(%q, %d) found=%v, want %v", tc.name, tc.line, got, tc.want)
		}
	}
}

func TestLoopExplicitDeclaration(t *testing.T) {
	_, identity := loadLoopIndex(t, "loop_identity.c")
	for _, tc := range []struct {
		name string
		line int
		want bool
	}{
		{"i", 14, true},
		{"i", 18, true},
		{"i", 21, true},
		{"outer", 25, true},
		{"inner", 26, false},
		{"shadow", 31, true},
		{"shadow", 32, false},
		{"a", 37, true},
		{"b", 37, true},
		{"q", 54, true},
		{"z", 61, false},
		{"c", 67, true},
	} {
		got := identity.loopForDeclaration(tc.name, observer.SourceLocation{File: identity.source, Line: tc.line}) != nil
		if got != tc.want {
			t.Errorf("loopForDeclaration(%q, %d)=%v, want %v", tc.name, tc.line, got, tc.want)
		}
	}
}

func TestLoopInferenceRejectsAmbiguity(t *testing.T) {
	_, identity := loadLoopIndex(t, "loop_identity.c")
	stop := observer.SourceLocation{File: identity.source, Line: 16}
	if identity.inferLoopDeclaration(dapVariable{Name: "i"}, stop, 2) != nil {
		t.Fatal("two visible same-name variables must not infer")
	}
	if identity.inferLoopDeclaration(dapVariable{Name: "i", EvaluateName: "parent.i"}, stop, 1) != nil {
		t.Fatal("qualified evaluateName must not infer")
	}
	explicit := dapVariable{Name: "i"}
	explicit.Extensions.Declaration = &struct {
		Path string
		Line int
	}{identity.source, 99}
	if identity.inferLoopDeclaration(explicit, stop, 1) != nil {
		t.Fatal("explicit declaration must take precedence")
	}
	if identity.inferLoopDeclaration(dapVariable{Name: "i"}, stop, 1) == nil {
		t.Fatal("unambiguous top-level loop must infer")
	}
}

func TestLoopGotoDisablesIndex(t *testing.T) {
	index, identity := loadLoopIndex(t, "loop_identity_goto.c")
	if !index.mainHasGoto {
		t.Fatal("goto fixture was not detected")
	}
	decl := identity.inferLoopDeclaration(dapVariable{Name: "i"}, observer.SourceLocation{File: identity.source, Line: 7}, 1)
	if decl != nil {
		t.Fatal("goto fixture must not infer loop identity")
	}
}
