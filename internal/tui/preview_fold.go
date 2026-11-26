package tui

import "fmt"

// Only consecutive actual indices with equal, observed, non-truncated
// displayed fields can be folded. The ≈ marker means preview equality, not
// equality of the underlying aggregate or unrecorded children.
func foldPreviewRows(t DataTable) (DataTable, bool) {
	if len(t.Headers) < 2 || t.Headers[0] != "下标" {
		return t, false // Preserve matrix geometry and named members.
	}
	sequence := len(t.Headers) == 3 && t.Headers[1] == "值" && t.Headers[2] == "类型"
	equal := func(a, b []Cell) bool {
		if len(a) != len(t.Headers) || len(b) != len(t.Headers) {
			return false
		}
		for i := 1; i < len(a); i++ {
			if sequence && i == 2 {
				if a[i].Text != b[i].Text {
					return false
				}
				continue
			}
			if !a[i].PreviewKnown || !b[i].PreviewKnown || a[i].Changed || b[i].Changed ||
				a[i].PreviewValue != b[i].PreviewValue {
				return false
			}
		}
		return true
	}
	out := DataTable{Headers: t.Headers}
	folded := false
	for i := 0; i < len(t.Rows); {
		first := t.Rows[i]
		if len(first) == 0 {
			out.Rows = append(out.Rows, first)
			i++
			continue
		}
		lo, valid := indexLabel(first[0].Text)
		j := i + 1
		for valid && j < len(t.Rows) && len(t.Rows[j]) > 0 {
			next, ok := indexLabel(t.Rows[j][0].Text)
			if !ok || next < lo || next-lo != j-i || !equal(first, t.Rows[j]) {
				break
			}
			j++
		}
		if j-i >= 3 {
			row := append([]Cell(nil), first...)
			row[0] = Cell{Text: fmt.Sprintf("[%d..%d]≈", lo, lo+j-i-1)}
			out.Rows = append(out.Rows, row)
			folded = true
			i = j
		} else {
			out.Rows = append(out.Rows, first)
			i++
		}
	}
	return out, folded
}
