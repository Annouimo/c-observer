// Package observer defines the provisional engine/UI contract.
// See docs/interface.md for semantics. Not yet a stable API.
package observer

import "context"

type Engine interface {
	Run(ctx context.Context, config LaunchConfig) RunResult
}

type LaunchConfig struct {
	Backend string // empty or auto, gdb, lldb
	SourcePath         string
	WorkDir            string
	VisibleSourcePaths []string
	SkipSourcePaths    []string
	SourceMap          map[string]string
	KeepLocations      []SourceLocation
	ArrayRanges        []ArrayRange
	Compiler           string
	Debugger           string
	CompilerArgs       []string
	Stdin              []byte
	ExpectedStdout     *[]byte
	Comparison         string // empty = raw bytes; otherwise Companion sample rule
	Limits             Limits
}

// ArrayRange selects one page of an array for the next run.
type ArrayRange struct {
	Path        string
	Declaration *SourceLocation
	Start       int
	Count       int
}

type Limits struct {
	MaxSnapshots         int
	TimeoutMillis        int64
	RequestTimeoutMillis int64
	MaxArrayElements     int
	MaxDepth             int
	MaxSnapshotNodes     int
	MaxOutputBytes       int
	MaxHistoryBytes      int64
}

type SourceFile struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

type Capabilities struct {
	Globals       bool `json:"globals"`
	ExplicitStops bool `json:"explicit_stops"`
}

type Session struct {
	Backend      string       `json:"backend"`
	Sources      []SourceFile `json:"sources"`
	Capabilities Capabilities `json:"capabilities"`
}

type SourceLocation struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	ReportedFile string `json:"reported_file,omitempty"`
	Function     string `json:"function,omitempty"`
}

type Frame struct {
	ID                string         `json:"id"`
	IdentityReliable  bool           `json:"identity_reliable"`
	Location          SourceLocation `json:"location"`
	Variables         []Variable     `json:"variables"`
	VariablesComplete bool           `json:"variables_complete"`
}

type VariableKind string

const (
	Scalar  VariableKind = "scalar"
	String  VariableKind = "string"
	Array   VariableKind = "array"
	Struct  VariableKind = "struct"
	Pointer VariableKind = "pointer"
	Unknown VariableKind = "unknown"
)

type ValueStatus string

const (
	Observed    ValueStatus = "observed"
	NotCaptured ValueStatus = "not_captured"
	Unavailable ValueStatus = "unavailable"
)

type VariableScope string

const (
	ParameterScope      VariableScope = "parameter"
	LocalScope          VariableScope = "local"
	FileStaticScope     VariableScope = "file_static"
	FunctionStaticScope VariableScope = "function_static"
	GlobalScope         VariableScope = "global"
)

type Variable struct {
	ID               string          `json:"id"`
	IdentityReliable bool            `json:"identity_reliable"`
	Name             string          `json:"name"`
	Scope            VariableScope   `json:"scope,omitempty"`
	Declaration      *SourceLocation `json:"declaration,omitempty"`
	Type             string          `json:"type"`
	Kind             VariableKind    `json:"kind"`
	Status           ValueStatus     `json:"status"`
	Value            string          `json:"value,omitempty"`
	Index            *int            `json:"index,omitempty"`
	ArrayLength      *int            `json:"array_length,omitempty"`
	Children         []Variable      `json:"children,omitempty"`
	ChildrenComplete bool            `json:"children_complete"`
	ValueTruncated   bool            `json:"value_truncated,omitempty"`
}

type OutputState struct {
	// Byte slices encode as base64 in JSON fixtures.
	Stdout          []byte `json:"stdout"`
	Stderr          []byte `json:"stderr"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
}

// OutputOffsets counts bytes retained in RunResult.Output at capture time.
type OutputOffsets struct {
	StdoutBytes int `json:"stdout_bytes"`
	StderrBytes int `json:"stderr_bytes"`
}

type Snapshot struct {
	ID              int            `json:"id"`
	Location        SourceLocation `json:"location"`
	Frames          []Frame        `json:"frames"`
	Globals         []Variable     `json:"globals,omitempty"`
	GlobalsComplete bool           `json:"globals_complete"`
	OutputOffsets   OutputOffsets  `json:"output_offsets"`
}

type FinishReason string

const (
	Exited          FinishReason = "exited"
	NonzeroExit     FinishReason = "nonzero_exit"
	Signaled        FinishReason = "signaled"
	TimedOut        FinishReason = "timed_out"
	RequestTimedOut FinishReason = "request_timed_out"
	InvalidConfig   FinishReason = "invalid_config"
	Cancelled       FinishReason = "cancelled"
	LimitReached    FinishReason = "limit_reached"
	CompileFailed   FinishReason = "compile_failed"
	BackendFailed   FinishReason = "backend_failed"
)

type RunResult struct {
	Session      Session      `json:"session"`
	Snapshots    []Snapshot   `json:"snapshots"`
	Diagnostics  []Diagnostic `json:"diagnostics,omitempty"`
	Reason       FinishReason `json:"reason"`
	ExitCode     *int         `json:"exit_code,omitempty"`
	Message      string       `json:"message,omitempty"`
	CleanupError string       `json:"cleanup_error,omitempty"`
	Output       OutputState  `json:"output"`
}

type Diagnostic struct {
	Severity string          `json:"severity"`
	Message  string          `json:"message"`
	Location *SourceLocation `json:"location,omitempty"`
}
