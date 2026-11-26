package engine

import (
	"os"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
)

// Experimental loop-control identity for the proven entry main frame.
//
// The index is deliberately limited to standalone C sources without #line
// mappings. It records only declarations that Clang places directly in the
// entry main body, so a loop in a helper or an assembled preset is never
// treated as a single main-frame object. A loop is eligible only when it is
// not nested inside another loop in main: such a loop is entered at most once
// during the single call to main, while an inner loop is re-created for every
// outer iteration and its dynamic instance cannot be proven from pauses alone.
// A goto anywhere in main disables the loop index because it can re-enter a
// loop without a new stack call.
type loopDecl struct {
	name     string
	declLine int
	loopLine int
	loopEnd  int
	topLevel bool
}

// localDecl is the lexical extent used only to reject ambiguous same-name
// candidates. It does not itself grant identity.
type localDecl struct {
	name       string
	scopeStart int
	scopeEnd   int
}

func (i *sourceIndex) indexMainBody(body astNode, text string, origins sourceOrigins) {
	if origins != nil {
		return
	}
	start, end := astRangeLines(body, text)
	if start == 0 || end == 0 {
		return
	}
	i.walkMain(body, text, start, end, 0)
}

func (i *sourceIndex) walkMain(node astNode, text string, scopeStart, scopeEnd, loopDepth int) {
	switch node.Kind {
	case "CompoundStmt":
		start, end := astRangeLines(node, text)
		if start == 0 || end == 0 {
			start, end = scopeStart, scopeEnd
		}
		for _, child := range node.Inner {
			i.walkMain(child, text, start, end, loopDepth)
		}
	case "DeclStmt":
		for _, decl := range node.Inner {
			if decl.Kind != "VarDecl" {
				continue
			}
			line := loopDeclLine(decl, i.mainSource, text)
			if line == 0 {
				continue
			}
			i.mainDecls = append(i.mainDecls, localDecl{name: decl.Name, scopeStart: line, scopeEnd: scopeEnd})
		}
	case "ForStmt":
		start, end := astRangeLines(node, text)
		if start == 0 || end == 0 {
			return
		}
		for _, child := range node.Inner {
			if child.Kind != "DeclStmt" {
				continue
			}
			for _, decl := range child.Inner {
				if decl.Kind != "VarDecl" {
					continue
				}
				line := loopDeclLine(decl, i.mainSource, text)
				if line == 0 {
					continue
				}
				i.mainLoops = append(i.mainLoops, loopDecl{
					name: decl.Name, declLine: line, loopLine: start, loopEnd: end, topLevel: loopDepth == 0,
				})
				i.mainDecls = append(i.mainDecls, localDecl{name: decl.Name, scopeStart: start, scopeEnd: end})
			}
		}
		// The last child is the loop body; condition and increment expressions
		// cannot declare a C object.
		for idx := len(node.Inner) - 1; idx >= 0; idx-- {
			child := node.Inner[idx]
			if child.Kind == "" || child.Kind == "DeclStmt" {
				continue
			}
			i.walkMain(child, text, start, end, loopDepth+1)
			break
		}
	case "WhileStmt", "DoStmt":
		start, end := astRangeLines(node, text)
		if start == 0 || end == 0 {
			start, end = scopeStart, scopeEnd
		}
		for _, child := range node.Inner {
			if child.Kind == "CompoundStmt" {
				i.walkMain(child, text, start, end, loopDepth+1)
			} else {
				i.walkMain(child, text, scopeStart, scopeEnd, loopDepth)
			}
		}
	case "IfStmt", "SwitchStmt":
		for _, child := range node.Inner {
			i.walkMain(child, text, scopeStart, scopeEnd, loopDepth)
		}
	case "GotoStmt":
		i.mainHasGoto = true
	default:
		for _, child := range node.Inner {
			i.walkMain(child, text, scopeStart, scopeEnd, loopDepth)
		}
	}
}

func astRangeLines(node astNode, text string) (int, int) {
	return astLocLine(node.Range.Begin, text), astLocLine(node.Range.End, text)
}

func astLocLine(loc astLoc, text string) int {
	if loc.Line > 0 {
		return loc.Line
	}
	if loc.Offset == nil || *loc.Offset < 0 || *loc.Offset > len(text) {
		return 0
	}
	return 1 + strings.Count(text[:*loc.Offset], "\n")
}

func loopDeclLine(decl astNode, source, text string) int {
	file, line := decl.sourceLine(source, text, nil)
	if file == "" || line == 0 {
		return 0
	}
	return line
}

// inferLoopDeclaration uses the Clang lexical extent only when there is one
// visible declaration of this name and the declaration is a top-level loop
// control variable. The debugger declaration, when present, always wins.
func loopIdentityEnabled() bool { return os.Getenv("COBS_LOOP_DISABLE") != "1" }

func (m mainIdentity) inferLoopDeclaration(raw dapVariable, stop observer.SourceLocation, count int) *observer.SourceLocation {
	if !loopIdentityEnabled() || m.hasGoto || raw.Extensions.Declaration != nil || count != 1 ||
		stop.File != m.source || stop.Line == 0 ||
		(raw.EvaluateName != "" && raw.EvaluateName != raw.Name) {
		return nil
	}
	loop := m.loopAt(raw.Name, stop.Line)
	if loop == nil {
		return nil
	}
	return &observer.SourceLocation{File: m.source, Line: loop.declLine}
}

func (m mainIdentity) loopAt(name string, line int) *loopDecl {
	matches := 0
	for _, decl := range m.decls {
		if decl.name == name && line >= decl.scopeStart && line <= decl.scopeEnd {
			matches++
		}
	}
	if matches != 1 {
		return nil
	}
	var found *loopDecl
	for i := range m.loops {
		loop := &m.loops[i]
		if loop.name != name || !loop.topLevel ||
			line <= loop.declLine || line < loop.loopLine || line > loop.loopEnd {
			continue
		}
		if found != nil {
			return nil
		}
		found = loop
	}
	return found
}

func (m mainIdentity) loopForDeclaration(name string, decl observer.SourceLocation) *loopDecl {
	if !loopIdentityEnabled() || m.hasGoto || decl.File != m.source || decl.Line == 0 {
		return nil
	}
	var found *loopDecl
	for i := range m.loops {
		loop := &m.loops[i]
		if loop.name == name && loop.declLine == decl.Line && loop.topLevel {
			if found != nil {
				return nil
			}
			found = loop
		}
	}
	return found
}
