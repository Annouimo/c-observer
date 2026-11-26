package tui

import "github.com/Routilet/c-observer/internal/observer"

// ReplayStep pairs an observed post-state with its actual chronological
// predecessor. A debugger stop is not evidence of a completed source line.
// The existing engine contract does not prove single-line attribution.
type ReplayStep struct {
	Before, After *observer.Snapshot
	Origin        observer.SourceLocation
}

func (r Replay) Step() ReplayStep {
	step := ReplayStep{Before: r.Previous(), After: r.Current()}
	if step.Before != nil && step.After != nil {
		step.Origin = step.Before.Location
	}
	return step
}

func (s ReplayStep) HasInterval() bool {
	return s.Before != nil && s.After != nil
}
