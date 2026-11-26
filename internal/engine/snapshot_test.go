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
