package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/Routilet/c-observer/internal/observer"
)

type Cell struct {
	Text           string
	Current        string
	Changed        bool
	Path, Raw, Key string
	Observed       bool
	Truncated      bool
	PreviewValue   string
	PreviewKnown   bool
}
type DataTable struct {
	Headers []string
	Rows    [][]Cell
}

func childRows(parent VariableRow, rows []VariableRow) []VariableRow {
	var children []VariableRow
	for _, r := range rows {
		if r.Parent == parent.Key {
			children = append(children, r)
		}
	}
	return children
}
func cellFor(r VariableRow) Cell {
	value := r.Value
	if r.Changed {
		value = r.Before + " → " + r.Value
	}
	if r.Detail != "" {
		if value != "" {
			value += " · "
		}
		value += r.Detail
	}
	if value == "" {
		value = "—"
	}
	return Cell{Text: value, Current: r.Value, Changed: r.Changed,
		Path: r.Path, Key: r.Key, Raw: SafeText(r.Raw.Value, true),
		Observed: r.Raw.Status == observer.Observed, Truncated: r.Raw.ValueTruncated,
		PreviewValue: r.Raw.Value, PreviewKnown: r.Raw.Status == observer.Observed && !r.Raw.ValueTruncated &&
			(r.Raw.Kind == observer.String || r.Raw.Kind == observer.Scalar || r.Raw.Kind == observer.Pointer)}
}

// tableFields expands named structure fields into shared columns, while leaving
// arrays as aggregates that can be inspected separately.
func tableFields(parent VariableRow, rows []VariableRow) []VariableRow {
	var out []VariableRow
	for _, field := range childRows(parent, rows) {
		if field.Raw.Kind == observer.Struct && field.Raw.Status == observer.Observed {
			nested := tableFields(field, rows)
			if len(nested) > 0 {
				for _, child := range nested {
					child.Name = field.Name + "." + child.Name
					out = append(out, child)
				}
				continue
			}
		}
		out = append(out, field)
	}
	return out
}

// TableFor renders only recorded children. A missing field is never treated as zero.
func TableFor(parent VariableRow, rows []VariableRow) DataTable {
	children := childRows(parent, rows)
	if parent.Raw.Kind == observer.Array {
		matrix := len(children) > 0
		for _, c := range children {
			if c.Raw.Kind != observer.Array {
				matrix = false
				break
			}
		}
		if matrix {
			indices := map[int]bool{}
			for _, row := range children {
				for _, c := range childRows(row, rows) {
					if c.Raw.Index != nil {
						indices[*c.Raw.Index] = true
					}
				}
			}
			var columns []int
			for i := range indices {
				columns = append(columns, i)
			}
			sort.Ints(columns)
			if len(columns) > 0 {
				t := DataTable{Headers: []string{"行 / 列"}}
				for _, i := range columns {
					t.Headers = append(t.Headers, fmt.Sprintf("[%d]", i))
				}
				for _, row := range children {
					line := []Cell{{Text: row.Name}}
					byIndex := map[int]VariableRow{}
					for _, c := range childRows(row, rows) {
						if c.Raw.Index != nil {
							byIndex[*c.Raw.Index] = c
						}
					}
					for _, i := range columns {
						if c, ok := byIndex[i]; ok {
							line = append(line, cellFor(c))
						} else {
							line = append(line, Cell{Text: "未记录"})
						}
					}
					t.Rows = append(t.Rows, line)
				}
				return t
			}
		}
		recordArray := len(children) > 0
		for _, c := range children {
			if c.Raw.Kind != observer.Struct {
				recordArray = false
				break
			}
		}
		if recordArray {
			t := DataTable{Headers: []string{"下标"}}
			fields := map[string]bool{}
			for _, c := range children {
				for _, f := range tableFields(c, rows) {
					if !fields[f.Name] {
						fields[f.Name] = true
						t.Headers = append(t.Headers, f.Name)
					}
				}
			}
			if len(t.Headers) > 1 {
				for _, c := range children {
					line := []Cell{{Text: c.Name}}
					byName := map[string]VariableRow{}
					for _, f := range tableFields(c, rows) {
						byName[f.Name] = f
					}
					for _, name := range t.Headers[1:] {
						if f, ok := byName[name]; ok {
							line = append(line, cellFor(f))
						} else {
							line = append(line, Cell{Text: "未记录"})
						}
					}
					t.Rows = append(t.Rows, line)
				}
				return t
			}
		}
		t := DataTable{Headers: []string{"下标", "值", "类型"}}
		for _, c := range children {
			t.Rows = append(t.Rows, []Cell{{Text: c.Name}, cellFor(c), {Text: c.Type}})
		}
		return t
	}
	if len(children) > 0 {
		t := DataTable{Headers: []string{"成员", "值", "类型"}}
		for _, c := range children {
			t.Rows = append(t.Rows, []Cell{{Text: c.Name}, cellFor(c), {Text: c.Type}})
		}
		return t
	}
	return DataTable{Headers: []string{"变量", "值", "类型"}, Rows: [][]Cell{{{Text: parent.Name}, cellFor(parent), {Text: parent.Type}}}}
}

type HistoryEntry struct {
	Position int
	Location observer.SourceLocation
	Text     string
	Changed  bool
}

func findRow(rows []VariableRow, key string) (VariableRow, bool) {
	for _, r := range rows {
		if r.Key == key {
			return r, true
		}
	}
	return VariableRow{}, false
}

// History is based only on reliable object identity and recorded pauses. Gaps
// start a new baseline; they are not inferred changes.
func (r Replay) History(target VariableRow) []HistoryEntry {
	if !target.Reliable {
		return nil
	}
	return r.historyIndex().Objects[target.Key]
}

// LineHistory lists recorded pauses, not exact execution counts.
func (r Replay) LineHistory(loc observer.SourceLocation) []HistoryEntry {
	return r.historyIndex().Lines[lineKey{loc.File, loc.Line}]
}

var arrayPath = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*(\.[A-Za-z_][A-Za-z_0-9]*|\[[0-9]+\])*$`)

func rangeFor(target VariableRow, declaration *observer.SourceLocation, startText, countText string, limit int) (observer.ArrayRange, error) {
	if target.Raw.Kind != observer.Array || !arrayPath.MatchString(target.Path) {
		return observer.ArrayRange{}, fmt.Errorf("此对象不能按路径重新采集")
	}
	start, err := strconv.Atoi(startText)
	if err != nil || start < 0 {
		return observer.ArrayRange{}, fmt.Errorf("起始下标必须是非负整数")
	}
	count, err := strconv.Atoi(countText)
	if limit <= 0 {
		limit = 16
	}
	if err != nil || count < 1 || count > limit {
		return observer.ArrayRange{}, fmt.Errorf("数量必须在 1～%d 之间", limit)
	}
	if target.Raw.ArrayLength != nil && start >= *target.Raw.ArrayLength {
		return observer.ArrayRange{}, fmt.Errorf("起始下标超出数组容量")
	}
	return observer.ArrayRange{Path: target.Path, Declaration: declaration, Start: start, Count: count}, nil
}
