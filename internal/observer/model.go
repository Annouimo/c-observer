// Package observer defines the provisional engine/UI contract.
// See docs/interface.md for semantics. Not yet a stable API.
package observer

import "context"

type Engine interface {
	Start(ctx context.Context, config LaunchConfig) (<-chan Event, error)
}

type LaunchConfig struct {
	SourcePath string
	WorkDir string
	VisibleSourcePaths []string
	SkipSourcePaths []string
	Compiler string
	Debugger string
	CompilerArgs []string
	Stdin []byte
	ExpectedStdout *[]byte
	Limits Limits
}

type Limits struct {
	MaxSnapshots int
	TimeoutMillis int64
	MaxArrayElements int
	MaxDepth int
	MaxOutputBytes int
	MaxHistoryBytes int64
}

type SourceFile struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

type Capabilities struct {
	Globals bool `json:"globals"`
	StableIdentity bool `json:"stable_identity"`
	InputConsumption bool `json:"input_consumption"`
	UserCallbacks bool `json:"user_callbacks"`
}

type Session struct {
	Backend string `json:"backend"`
	Sources []SourceFile `json:"sources"`
	Capabilities Capabilities `json:"capabilities"`
}

type SourceLocation struct {
	File string `json:"file"`
	Line int `json:"line"`
	Function string `json:"function,omitempty"`
}

type Frame struct {
	ID string `json:"id"`
	IdentityReliable bool `json:"identity_reliable"`
	Location SourceLocation `json:"location"`
	Variables []Variable `json:"variables"`
}

type VariableKind string

const (
	Scalar VariableKind = "scalar"
	String VariableKind = "string"
	Array VariableKind = "array"
	Struct VariableKind = "struct"
	Pointer VariableKind = "pointer"
	Unknown VariableKind = "unknown"
)

type ValueStatus string

const (
	Observed ValueStatus = "observed"
	NotCaptured ValueStatus = "not_captured"
	Unavailable ValueStatus = "unavailable"
)

type Variable struct {
	ID string `json:"id"`
	IdentityReliable bool `json:"identity_reliable"`
	Name string `json:"name"`
	Type string `json:"type"`
	Kind VariableKind `json:"kind"`
	Status ValueStatus `json:"status"`
	Value string `json:"value,omitempty"`
	Index *int `json:"index,omitempty"`
	ArrayLength *int `json:"array_length,omitempty"`
	Children []Variable `json:"children,omitempty"`
	ChildrenComplete bool `json:"children_complete"`
	ValueTruncated bool `json:"value_truncated,omitempty"`
}

type OutputState struct {
	// Byte slices encode as base64 in JSON fixtures.
	Stdout []byte `json:"stdout"`
	Stderr []byte `json:"stderr"`
	StdoutTruncated bool `json:"stdout_truncated,omitempty"`
	StderrTruncated bool `json:"stderr_truncated,omitempty"`
	StdinConsumed *int64 `json:"stdin_consumed,omitempty"`
}

type Snapshot struct {
	ID int `json:"id"`
	Location SourceLocation `json:"location"`
	Frames []Frame `json:"frames"`
	Globals []Variable `json:"globals,omitempty"`
	Output OutputState `json:"output"`
}

type FinishReason string

const (
	Exited FinishReason = "exited"
	NonzeroExit FinishReason = "nonzero_exit"
	Signaled FinishReason = "signaled"
	TimedOut FinishReason = "timed_out"
	Cancelled FinishReason = "cancelled"
	LimitReached FinishReason = "limit_reached"
	CompileFailed FinishReason = "compile_failed"
	BackendFailed FinishReason = "backend_failed"
)

type RunResult struct {
	Reason FinishReason `json:"reason"`
	ExitCode *int `json:"exit_code,omitempty"`
	Message string `json:"message,omitempty"`
	Output OutputState `json:"output"`
}

type Progress struct {
	Captured int `json:"captured"`
	Message string `json:"message,omitempty"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Message string `json:"message"`
	Location *SourceLocation `json:"location,omitempty"`
}

type EventKind string

const (
	StartedEvent EventKind = "started"
	SnapshotEvent EventKind = "snapshot"
	ProgressEvent EventKind = "progress"
	DiagnosticEvent EventKind = "diagnostic"
	FinishedEvent EventKind = "finished"
)

type Event struct {
	Kind EventKind `json:"kind"`
	Session *Session `json:"session,omitempty"`
	Snapshot *Snapshot `json:"snapshot,omitempty"`
	Progress *Progress `json:"progress,omitempty"`
	Diagnostic *Diagnostic `json:"diagnostic,omitempty"`
	Result *RunResult `json:"result,omitempty"`
}
