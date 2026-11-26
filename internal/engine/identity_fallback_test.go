package engine

import (
	"testing"

	"github.com/Routilet/c-observer/internal/observer"
)

func TestEntryDeclarationFallback(t *testing.T) {
	m := mainIdentity{source: "student.c", lines: map[string]int{"birthdays": 11}}
	raw := dapVariable{Name: "birthdays", EvaluateName: "birthdays"}
	stop := observer.SourceLocation{File: "student.c", Line: 19}
	declaration := m.inferDeclaration(raw, stop, 1)
	if declaration == nil || declaration.File != stop.File || declaration.Line != 11 {
		t.Fatalf("unambiguous mapped local: %+v", declaration)
	}
	if m.inferDeclaration(raw, stop, 2) != nil {
		t.Fatal("duplicate debugger name must remain ambiguous")
	}
	stop.Line = 11
	if m.inferDeclaration(raw, stop, 1) != nil {
		t.Fatal("fallback applied at the declaration line")
	}
}
