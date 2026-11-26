package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
)

type stackFrame struct {
	PC       string
	ID       int
	Location observer.SourceLocation
}

func (c *capture) location(path string, line int, function string) observer.SourceLocation {
	loc := observer.SourceLocation{Line: max(line, 0), Function: function}
	if path == "" {
		return loc
	}
	candidates := []string{path, absolute(c.dir, path)}
	// DWARF/DAP may report an absolute path for a relative #line origin.
	// Resolve only against the working directory; never guess by basename.
	if relative, err := filepath.Rel(c.dir, absolute(c.dir, path)); err == nil {
		candidates = append(candidates, relative, filepath.ToSlash(relative))
	}
	for _, candidate := range candidates {
		if target, ok := c.config.SourceMap[candidate]; ok {
			target = absolute(c.dir, target)
			if _, visible := c.files[target]; visible {
				loc.File = target
				return loc
			}
		}
	}
	resolved := absolute(c.dir, path)
	if _, visible := c.files[resolved]; visible {
		loc.File = resolved
		return loc
	}
	if canonical, err := filepath.EvalSymlinks(resolved); err == nil {
		for _, source := range c.result.Session.Sources {
			if candidate, err := filepath.EvalSymlinks(source.Path); err == nil && canonical == candidate {
				loc.File = source.Path
				return loc
			}
		}
	}
	loc.ReportedFile = path
	return loc
}

func (c *capture) skip(file string) bool {
	for _, path := range c.config.SkipSourcePaths {
		if absolute(c.dir, path) == file {
			return true
		}
	}
	return false
}

func (c *capture) explicit(actual observer.SourceLocation) bool {
	for _, loc := range c.config.KeepLocations {
		if loc.Line == actual.Line && ((actual.File != "" && absolute(c.dir, loc.File) == actual.File) ||
			(loc.ReportedFile != "" && loc.ReportedFile == actual.ReportedFile)) {
			return true
		}
	}
	return false
}

func (c *capture) keepLocations() error {
	lines := map[string][]map[string]int{}
	for _, loc := range c.config.KeepLocations {
		if loc.File == "" || loc.Line < 1 {
			c.result.Diagnostics = append(c.result.Diagnostics, observer.Diagnostic{Severity: "warning", Message: "invalid explicit stop location"})
			continue
		}
		file := absolute(c.dir, loc.File)
		if loc.ReportedFile != "" {
			file = loc.ReportedFile
		}
		lines[file] = append(lines[file], map[string]int{"line": loc.Line})
	}
	for file, points := range lines {
		var result struct {
			Breakpoints []struct {
				Verified bool
				Message  string
			}
		}
		if err := c.ask("setBreakpoints", map[string]any{"source": map[string]string{"path": file}, "breakpoints": points}, &result); err != nil {
			return err
		}
		for i, requested := range points {
			if i >= len(result.Breakpoints) || !result.Breakpoints[i].Verified {
				loc := c.location(file, requested["line"], "")
				c.result.Diagnostics = append(c.result.Diagnostics, observer.Diagnostic{
					Severity: "warning", Message: "could not bind explicit stop", Location: &loc})
			}
		}
	}
	return nil
}

func (c *capture) stack(thread int) ([]stackFrame, error) {
	var body struct {
		StackFrames []struct {
			InstructionPointerReference string
			ID                          int
			Name                        string
			Source                      struct{ Path string }
			Line                        int
		}
	}
	if err := c.ask("stackTrace", map[string]any{"threadId": thread, "levels": min(32, c.limits.MaxSnapshotNodes)}, &body); err != nil {
		return nil, err
	}
	frames := make([]stackFrame, 0, len(body.StackFrames))
	for _, f := range body.StackFrames {
		frames = append(frames, stackFrame{ID: f.ID, PC: f.InstructionPointerReference, Location: c.location(f.Source.Path, f.Line, f.Name)})
	}
	return frames, nil
}

func (c *capture) snapshot(frames []stackFrame) (observer.Snapshot, error) {
	snapshot := observer.Snapshot{ID: len(c.result.Snapshots)}
	if len(frames) > 0 {
		snapshot.Location = frames[0].Location
	}
	nodes := c.limits.MaxSnapshotNodes
	seenGlobals := make(map[declarationKey]bool)
	entry := -1
	if len(frames) < min(32, c.limits.MaxSnapshotNodes) {
		entry = c.identity.entryFrame(frames, snapshot.ID == 0 && snapshot.Location.Function == "main")
	}
	for depth, raw := range frames {
		frame := observer.Frame{ID: fmt.Sprintf("%d:%d", snapshot.ID, depth), Location: raw.Location}
		if depth == entry {
			frame.ID, frame.IdentityReliable = "entry-main", true
		}
		if raw.Location.File != "" && !c.skip(raw.Location.File) && nodes > 0 {
			var body struct {
				Scopes []struct {
					Name               string
					VariablesReference int
				}
			}
			if err := c.ask("scopes", map[string]any{"frameId": raw.ID}, &body); err != nil {
				return snapshot, err
			}
			frame.VariablesComplete = true
			for _, scope := range body.Scopes {
				name := strings.ToLower(scope.Name)
				if (name == "globals" || name == "statics") && len(c.index.globals) > 0 {
					if err := c.collectGlobals(scope.VariablesReference, &nodes, seenGlobals, &snapshot); err != nil {
						return snapshot, err
					}
					continue
				}
				if name != "locals" && name != "arguments" {
					continue
				}
				vars, err := c.variables(scope.VariablesReference, 0, 0)
				if err != nil {
					return snapshot, err
				}
				var localNames map[string]int
				if frame.IdentityReliable && name == "locals" {
					localNames = make(map[string]int, len(vars))
					for _, variable := range vars {
						localNames[variableName(variable)]++
					}
				}
				for i, variable := range vars {
					if nodes == 0 {
						frame.VariablesComplete = false
						break
					}
					var declaration *observer.SourceLocation
					if localNames != nil {
						declaration = c.identity.inferDeclaration(variable, raw.Location, localNames[variable.Name])
					}
					v, err := c.variable(variable, fmt.Sprintf("%s:%s:%d", frame.ID, name, i), variableName(variable), declaration, 0, &nodes)
					if err != nil {
						return snapshot, err
					}
					if name == "arguments" {
						v.Scope = observer.ParameterScope
					}
					if localNames != nil {
						c.identity.mark(&v, variableName(variable))
					}
					frame.Variables = append(frame.Variables, v)
				}
			}
		}
		snapshot.Frames = append(snapshot.Frames, frame)
	}
	stdout, err := outputSize(c.output, c.limits.MaxOutputBytes)
	if err != nil {
		return snapshot, err
	}
	stderr, err := outputSize(c.stderr, c.limits.MaxOutputBytes)
	if err != nil {
		return snapshot, err
	}
	snapshot.OutputOffsets = observer.OutputOffsets{StdoutBytes: stdout, StderrBytes: stderr}
	return snapshot, nil
}

func (c *capture) collectGlobals(ref int, nodes *int, seen map[declarationKey]bool, snapshot *observer.Snapshot) error {
	if *nodes == 0 {
		return nil
	}
	vars, err := c.variables(ref, 0, 0)
	if err != nil {
		return err
	}
	for _, raw := range vars {
		decl := raw.Extensions.Declaration
		if decl == nil {
			continue
		}
		key := declarationKey{variableName(raw), c.location(decl.Path, decl.Line, "").File, decl.Line}
		scope := c.index.globals[key]
		if scope == "" || seen[key] {
			continue
		}
		if *nodes == 0 {
			break
		}
		id := fmt.Sprintf("static-storage:%s:%d:%s", key.file, key.line, key.name)
		v, err := c.variable(raw, id, key.name, nil, 0, nodes)
		if err != nil {
			return err
		}
		v.Scope = scope
		markObject(&v, id)
		seen[key] = true
		c.result.Session.Capabilities.Globals = true
		snapshot.Globals = append(snapshot.Globals, v)
	}
	return nil
}

func outputSize(path string, limit int) (int, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return int(min(info.Size(), int64(limit))), nil
}

type dapVariable struct {
	Name               string
	EvaluateName       string
	Type               string
	Value              *string
	VariablesReference int
	IndexedVariables   *int
	NamedVariables     *int
	Extensions         struct {
		Declaration *struct {
			Path string
			Line int
		}
	} `json:"$__lldb_extensions"`
}

func variableName(v dapVariable) string {
	if v.EvaluateName != "" && !strings.Contains(v.Name, " @ ") {
		return v.EvaluateName
	}
	name, _, _ := strings.Cut(v.Name, " @ ")
	return name
}

var arrayType = regexp.MustCompile(`^[^()]+?\[([0-9]+)\](?:\s*\[[0-9]+\])*$`)

func (c *capture) variables(ref, start, count int) ([]dapVariable, error) {
	var body struct{ Variables []dapVariable }
	args := map[string]any{"variablesReference": ref}
	if count > 0 {
		args["start"], args["count"] = start, count
	}
	err := c.ask("variables", args, &body)
	return body.Variables, err
}

func (c *capture) variable(raw dapVariable, id, path string, rootDeclaration *observer.SourceLocation, depth int, nodes *int) (observer.Variable, error) {
	*nodes--
	v := observer.Variable{ID: id, Name: raw.Name, Type: raw.Type, Kind: observer.Scalar, Status: observer.Observed, ChildrenComplete: true}
	if raw.Value == nil {
		v.Status = observer.Unavailable
	} else {
		v.Value = *raw.Value
		if v.Value == "<unavailable>" || v.Value == "<optimized out>" {
			v.Status = observer.Unavailable
		}
		if len(v.Value) > 1024 {
			v.Value, v.ValueTruncated = strings.ToValidUTF8(v.Value[:1024], "�"), true
		}
	}
	if raw.Extensions.Declaration != nil {
		loc := c.location(raw.Extensions.Declaration.Path, raw.Extensions.Declaration.Line, "")
		v.Declaration = &loc
	}
	if depth == 0 {
		if v.Declaration == nil {
			v.Declaration = rootDeclaration
		}
		rootDeclaration = v.Declaration
	}
	isArray := false
	if match := arrayType.FindStringSubmatch(raw.Type); match != nil {
		isArray = true
		v.Kind = observer.Array
		if n, err := strconv.Atoi(match[1]); err == nil {
			v.ArrayLength = &n
		}
		if strings.HasPrefix(raw.Type, "char[") || strings.HasPrefix(raw.Type, "char [") {
			v.Kind = observer.String
			// LLDB applies its own string summary limit.
			v.ValueTruncated = v.ValueTruncated || strings.HasSuffix(v.Value, "...")
		}
	} else if strings.Contains(raw.Type, "*") {
		v.Kind = observer.Pointer
		v.ChildrenComplete = raw.VariablesReference == 0
		return v, nil
	} else if raw.VariablesReference != 0 {
		v.Kind = observer.Struct
	}
	if raw.VariablesReference == 0 {
		if isArray {
			v.ChildrenComplete = v.ArrayLength != nil && *v.ArrayLength == 0
		}
		return v, nil
	}
	v.ChildrenComplete = false
	if depth >= c.limits.MaxDepth || *nodes == 0 {
		return v, nil
	}
	start, count := 0, *nodes
	if isArray {
		count = min(count, c.limits.MaxArrayElements)
		if page := arrayRange(c.config.ArrayRanges, path, rootDeclaration); page != nil {
			start, count = page.Start, min(count, page.Count)
		}
		if v.ArrayLength != nil {
			count = min(count, *v.ArrayLength-start)
		}
	}
	if count <= 0 {
		return v, nil
	}
	children, err := c.variables(raw.VariablesReference, start, count)
	if err != nil {
		return v, err
	}
	for i, child := range children {
		if i == count || *nodes == 0 {
			break
		}
		name := variableName(child)
		childPath := path + "." + name
		var index *int
		if isArray {
			n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(child.Name, "["), "]"))
			if err != nil || n < start || n-start >= count {
				continue
			}
			index = &n
			childPath = fmt.Sprintf("%s[%d]", path, n)
		}
		observed, err := c.variable(child, fmt.Sprintf("%s/%d", id, i), childPath, rootDeclaration, depth+1, nodes)
		if err != nil {
			return v, err
		}
		observed.Index = index
		v.Children = append(v.Children, observed)
	}
	total := raw.NamedVariables
	if isArray {
		total = v.ArrayLength
		if total == nil {
			total = raw.IndexedVariables
		}
	}
	v.ChildrenComplete = start == 0 && total != nil && len(v.Children) == *total
	return v, nil
}

// A declaration-qualified selection overrides a name-only default. For
// equally specific entries, use the most recently supplied selection.
func arrayRange(ranges []observer.ArrayRange, path string, declaration *observer.SourceLocation) *observer.ArrayRange {
	var fallback *observer.ArrayRange
	for i := len(ranges) - 1; i >= 0; i-- {
		page := &ranges[i]
		if page.Path != path {
			continue
		}
		if page.Declaration != nil {
			if sameDeclaration(page.Declaration, declaration) {
				return page
			}
		} else if fallback == nil {
			fallback = page
		}
	}
	return fallback
}

func sameDeclaration(a, b *observer.SourceLocation) bool {
	return b != nil && a.File != "" && a.File == b.File && a.Line == b.Line
}

var variablePath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*|\[[0-9]+\])*$`)

func validateRanges(ranges []observer.ArrayRange) error {
	for _, r := range ranges {
		if !variablePath.MatchString(r.Path) || r.Start < 0 || r.Count <= 0 {
			return fmt.Errorf("invalid array range for %q", r.Path)
		}
	}
	return nil
}
