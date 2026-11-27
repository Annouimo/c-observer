package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
)

// Only direct declarations in the entry main's outer block have a proven
// lifetime here. Clang supplies the lexical boundary and DAP supplies a
// complete stack identifying the still-active entry call.
type mainIdentity struct {
	source string
	lines  map[string]int
	caller string
}

type astNode struct {
	Kind, Name, StorageClass string
	Loc                      struct {
		Offset       *int
		File         string
		PresumedFile string
		PresumedLine int
		ExpansionLoc *struct{}
		SpellingLoc  *struct{}
		IncludedFrom *struct{}
	}
	Type  struct{ QualType, DesugaredQualType string }
	Inner []astNode
}

type declarationKey struct {
	name string
	file string
	line int
}

type sourceIndex struct {
	functions  []declarationKey
	mainLines  map[string]int
	mainSource string
	globals    map[declarationKey]observer.VariableScope
}

func inspectSource(ctx context.Context, compiler, dir, source, text string, args []string, origins sourceOrigins) sourceIndex {
	cmd := exec.CommandContext(ctx, compiler, append(append([]string(nil), args...),
		"-fsyntax-only", "-Xclang", "-ast-dump=json", source)...)
	cmd.Dir = dir
	configureProcess(cmd)
	cmd.Cancel = func() error { return killProcessGroup(cmd) }
	cmd.WaitDelay = cleanupTimeout
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return sourceIndex{}
	}
	// Ignore headers and preset definitions as each top-level declaration is
	// decoded. An expanded C Exam Kit preset can exceed 8 MiB of AST JSON.
	input := &io.LimitedReader{R: stdout, N: 64 << 20}
	decoder := json.NewDecoder(input)
	index := sourceIndex{globals: make(map[declarationKey]observer.VariableScope)}
	parseErr := readASTDeclarations(decoder, func(root astNode) {
		file, line := root.sourceLine(source, text, origins)
		if line == 0 {
			return
		}
		if root.Kind == "VarDecl" {
			switch root.StorageClass {
			case "static":
				index.addGlobal(declarationKey{root.Name, file, line}, observer.FileStaticScope)
			case "":
				index.addGlobal(declarationKey{root.Name, file, line}, observer.GlobalScope)
			}
			return
		}
		if root.Kind != "FunctionDecl" {
			return
		}
		for _, body := range root.Inner {
			if body.Kind == "CompoundStmt" {
				index.functions = append(index.functions, declarationKey{root.Name, file, line})
				index.findFunctionStatics(body, source, text, origins)
				break
			}
		}
		if root.Name != "main" {
			return
		}
		for _, body := range root.Inner {
			if body.Kind != "CompoundStmt" {
				continue
			}
			lines := map[string]int{}
			for _, statement := range body.Inner {
				if statement.Kind != "DeclStmt" {
					continue
				}
				for _, decl := range statement.Inner {
					declFile, declLine := decl.sourceLine(source, text, origins)
					if decl.Kind != "VarDecl" || decl.StorageClass != "" || declFile != file || declLine == 0 {
						continue
					}
					typeName := decl.Type.DesugaredQualType
					if typeName == "" {
						typeName = decl.Type.QualType
					}
					if strings.Contains(typeName, "[") && !arrayType.MatchString(typeName) {
						continue // Variable-length or otherwise ambiguous array type.
					}
					lines[decl.Name] = declLine
				}
			}
			counts := map[string]int{}
			countLocalNames(body, counts)
			for name := range lines {
				if counts[name] != 1 {
					delete(lines, name) // DAP may report only a line, with no column.
				}
			}
			index.mainLines, index.mainSource = lines, file
		}
	})
	_ = stdout.Close()
	if cmd.Wait() != nil || parseErr != nil || input.N == 0 {
		return sourceIndex{}
	}
	return index
}

// Decode the single translation unit one declaration at a time, without
// retaining AST nodes from system headers for the remainder of the run.
func readASTDeclarations(decoder *json.Decoder, visit func(astNode)) error {
	expect := func(want json.Token) error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if token != want {
			return fmt.Errorf("expected AST token %v", want)
		}
		return nil
	}
	if err := expect(json.Delim('{')); err != nil {
		return err
	}
	found := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return err
		}
		if key != "inner" {
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return err
			}
			continue
		}
		if err := expect(json.Delim('[')); err != nil {
			return err
		}
		for decoder.More() {
			var declaration astNode
			if err := decoder.Decode(&declaration); err != nil {
				return err
			}
			visit(declaration)
		}
		if err := expect(json.Delim(']')); err != nil {
			return err
		}
		found = true
	}
	if err := expect(json.Delim('}')); err != nil {
		return err
	}
	if token, err := decoder.Token(); !found || token != nil || err != io.EOF {
		return fmt.Errorf("incomplete or unexpected Clang AST output")
	}
	return nil
}

func (node astNode) sourceLine(source, text string, origins sourceOrigins) (string, int) {
	loc := node.Loc
	if node.Name == "" || loc.IncludedFrom != nil || loc.ExpansionLoc != nil ||
		loc.SpellingLoc != nil || (loc.File != "" && loc.File != source) || loc.Offset == nil ||
		*loc.Offset < 0 || *loc.Offset >= len(text) || !strings.HasPrefix(text[*loc.Offset:], node.Name) {
		return "", 0
	}
	if origins == nil {
		return source, 1 + strings.Count(text[:*loc.Offset], "\n")
	}
	return origins.resolve(*loc.Offset, loc.PresumedFile, loc.PresumedLine)
}

func (i sourceIndex) addGlobal(key declarationKey, scope observer.VariableScope) {
	if _, exists := i.globals[key]; exists {
		i.globals[key] = "" // This location does not distinguish the declarations.
		return
	}
	i.globals[key] = scope
}

func (i sourceIndex) findFunctionStatics(node astNode, source, text string, origins sourceOrigins) {
	if node.Kind == "VarDecl" && node.StorageClass == "static" {
		if file, line := node.sourceLine(source, text, origins); line > 0 {
			i.addGlobal(declarationKey{node.Name, file, line}, observer.FunctionStaticScope)
		}
	}
	for _, child := range node.Inner {
		i.findFunctionStatics(child, source, text, origins)
	}
}

func countLocalNames(node astNode, counts map[string]int) {
	if node.Kind == "VarDecl" {
		counts[node.Name]++
	}
	for _, child := range node.Inner {
		countLocalNames(child, counts)
	}
}

func (m *mainIdentity) entryFrame(frames []stackFrame, first bool) int {
	if len(m.lines) == 0 {
		return -1
	}
	for depth := len(frames) - 2; depth >= 0; depth-- {
		frame := frames[depth]
		if frame.Location.Function != "main" || frame.Location.File != m.source {
			continue
		}
		caller := frames[depth+1].PC
		if first {
			m.caller = caller
		}
		if caller != "" && caller == m.caller {
			return depth
		}
		return -1
	}
	return -1
}

// inferDeclaration is limited to a unique ordinary local in the proven
// entry frame. An explicit debugger declaration, even one that fails source
// mapping, takes precedence. Neither the name alone nor a DAP reference proves
// identity: the AST excludes shadowing and shorter lifetimes.
func (m mainIdentity) inferDeclaration(raw dapVariable, stop observer.SourceLocation, count int) *observer.SourceLocation {
	line := m.lines[raw.Name]
	if raw.Extensions.Declaration != nil || count != 1 || line == 0 ||
		stop.File != m.source || stop.Line <= line ||
		(raw.EvaluateName != "" && raw.EvaluateName != raw.Name) {
		return nil
	}
	return &observer.SourceLocation{File: m.source, Line: line}
}

func (m mainIdentity) mark(v *observer.Variable, name string) {
	if v.Declaration == nil || v.Declaration.File != m.source || m.lines[name] != v.Declaration.Line ||
		v.Declaration.Line == 0 {
		return
	}
	v.Scope = observer.LocalScope
	markObject(v, fmt.Sprintf("entry-main:%d:%s", v.Declaration.Line, name))
}

func markObject(v *observer.Variable, id string) {
	v.ID, v.IdentityReliable = id, true
	if v.Kind == observer.Pointer {
		return // The address can change; pointee identity is not pointer identity.
	}
	keys := make(map[string]int, len(v.Children))
	for _, child := range v.Children {
		keys[memberKey(child)]++
	}
	for i := range v.Children {
		child := &v.Children[i]
		key := memberKey(*child)
		if key != "" && keys[key] == 1 {
			markObject(child, id+"/"+key)
		}
	}
}

func memberKey(v observer.Variable) string {
	if v.Index != nil {
		return fmt.Sprintf("[%d]", *v.Index)
	}
	if v.Name == "" {
		return ""
	}
	return "." + v.Name
}


// The adapter's global/static scope plus a unique indexed file-scope name
// permits a display match when declaration metadata is absent. It never
// upgrades the object to reliable identity or includes function statics.
func (i sourceIndex) uniqueGlobal(name string) (declarationKey, observer.VariableScope) {
	var key declarationKey
	var scope observer.VariableScope
	for k, s := range i.globals {
		if k.name != name {
			continue
		}
		if scope != "" || (s != observer.GlobalScope && s != observer.FileStaticScope) {
			return declarationKey{}, ""
		}
		key, scope = k, s
	}
	return key, scope
}
