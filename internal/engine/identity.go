package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Annouimo/c-observer/internal/observer"
)

// Only direct declarations in the entry main's outer block have a proven
// lifetime here. Clang supplies the lexical boundary; DAP supplies declaration
// locations and a complete stack identifying the still-active entry call.
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
		ExpansionLoc *struct{}
		SpellingLoc  *struct{}
		IncludedFrom *struct{}
	}
	Type  struct{ QualType, DesugaredQualType string }
	Inner []astNode
}

type declarationKey struct {
	name string
	line int
}

type sourceIndex struct {
	functions []string
	mainLines map[string]int
	globals   map[declarationKey]observer.VariableScope
}

func inspectSource(ctx context.Context, compiler, dir, source, text string, args []string) sourceIndex {
	cmd := exec.CommandContext(ctx, compiler, append(append([]string(nil), args...),
		"-fsyntax-only", "-Xclang", "-ast-dump=json", source)...)
	cmd.Dir = dir
	configureProcess(cmd)
	cmd.Cancel = func() error { return killProcessGroup(cmd) }
	cmd.WaitDelay = cleanupTimeout
	out := &cappedBuffer{limit: 8 << 20}
	cmd.Stdout = out
	if err := cmd.Run(); err != nil {
		return sourceIndex{}
	}
	var unit astNode
	if json.Unmarshal([]byte(out.String()), &unit) != nil {
		return sourceIndex{}
	}
	index := sourceIndex{globals: make(map[declarationKey]observer.VariableScope)}
	for _, root := range unit.Inner {
		line := root.sourceLine(source, text)
		if line == 0 {
			continue
		}
		if root.Kind == "VarDecl" {
			switch root.StorageClass {
			case "static":
				index.addGlobal(declarationKey{root.Name, line}, observer.FileStaticScope)
			case "":
				index.addGlobal(declarationKey{root.Name, line}, observer.GlobalScope)
			}
			continue
		}
		if root.Kind != "FunctionDecl" {
			continue
		}
		for _, body := range root.Inner {
			if body.Kind == "CompoundStmt" {
				index.functions = append(index.functions, root.Name)
				index.findFunctionStatics(body, source, text)
				break
			}
		}
		if root.Name != "main" {
			continue
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
					loc := decl.Loc
					if decl.Kind != "VarDecl" || decl.StorageClass != "" || loc.Offset == nil ||
						loc.ExpansionLoc != nil || loc.SpellingLoc != nil || loc.IncludedFrom != nil ||
						(loc.File != "" && loc.File != source) || *loc.Offset < 0 || *loc.Offset >= len(text) {
						continue
					}
					typeName := decl.Type.DesugaredQualType
					if typeName == "" {
						typeName = decl.Type.QualType
					}
					if strings.Contains(typeName, "[") && !arrayType.MatchString(typeName) {
						continue // Variable-length or otherwise ambiguous array type.
					}
					lines[decl.Name] = 1 + strings.Count(text[:*loc.Offset], "\n")
				}
			}
			counts := map[string]int{}
			countLocalNames(body, counts)
			for name := range lines {
				if counts[name] != 1 {
					delete(lines, name) // DAP may report only a line, with no column.
				}
			}
			index.mainLines = lines
		}
	}
	return index
}

func (node astNode) sourceLine(source, text string) int {
	loc := node.Loc
	if node.Name == "" || loc.IncludedFrom != nil || loc.ExpansionLoc != nil ||
		loc.SpellingLoc != nil || (loc.File != "" && loc.File != source) || loc.Offset == nil ||
		*loc.Offset < 0 || *loc.Offset >= len(text) || !strings.HasPrefix(text[*loc.Offset:], node.Name) {
		return 0
	}
	return 1 + strings.Count(text[:*loc.Offset], "\n")
}

func (i sourceIndex) addGlobal(key declarationKey, scope observer.VariableScope) {
	if _, exists := i.globals[key]; exists {
		i.globals[key] = "" // This location does not distinguish the declarations.
		return
	}
	i.globals[key] = scope
}

func (i sourceIndex) findFunctionStatics(node astNode, source, text string) {
	if node.Kind == "VarDecl" && node.StorageClass == "static" {
		if line := node.sourceLine(source, text); line > 0 {
			i.addGlobal(declarationKey{node.Name, line}, observer.FunctionStaticScope)
		}
	}
	for _, child := range node.Inner {
		i.findFunctionStatics(child, source, text)
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
