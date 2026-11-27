package record

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Routilet/c-observer/internal/observer"
)

const RecordVersion = 1
const maxRecordBytes = 128 << 20

// ObservationRecord is a portable envelope around the unchanged engine result.
// Byte slices use JSON base64 to preserve arbitrary input/output bytes.
type ObservationRecord struct {
	Format          string             `json:"format"`
	SchemaVersion   int                `json:"schema_version"`
	ObserverVersion string             `json:"observer_version"`
	RecordedAt      time.Time          `json:"recorded_at"`
	Platform        string             `json:"platform"`
	Arch            string             `json:"arch"`
	BackendRequest  string             `json:"backend_request,omitempty"`
	CompilerRequest string             `json:"compiler_request,omitempty"`
	DebuggerRequest string             `json:"debugger_request,omitempty"`
	Stdin           []byte             `json:"stdin"`
	ExpectedStdout  *[]byte            `json:"expected_stdout"`
	Comparison      string             `json:"comparison,omitempty"`
	Run             observer.RunResult `json:"run"`
}

func Capture(version string, cfg observer.LaunchConfig, run observer.RunResult) ObservationRecord {
	if version == "" {
		version = "dev"
	}
	record := ObservationRecord{
		Format: "c-observer-record", SchemaVersion: RecordVersion,
		ObserverVersion: version, RecordedAt: time.Now().UTC(),
		Platform: runtime.GOOS, Arch: runtime.GOARCH,
		BackendRequest: cfg.Backend, CompilerRequest: cfg.Compiler, DebuggerRequest: cfg.Debugger,
		Stdin: append([]byte{}, cfg.Stdin...), Comparison: cfg.Comparison, Run: run,
	}
	if cfg.ExpectedStdout != nil {
		b := append([]byte{}, (*cfg.ExpectedStdout)...)
		record.ExpectedStdout = &b
	}
	return record
}

func (r ObservationRecord) Validate() error {
	if r.Format != "c-observer-record" || r.SchemaVersion != RecordVersion {
		return fmt.Errorf("unsupported observation record format or schema version")
	}
	if r.ObserverVersion == "" || r.RecordedAt.IsZero() || r.Platform == "" || r.Arch == "" {
		return fmt.Errorf("missing observation record metadata")
	}
	if r.Run.Session.Backend == "synthetic" {
		return fmt.Errorf("synthetic fixtures are not observation records")
	}
	return ValidateRun(r.Run)
}

func DecodeRecord(reader io.Reader) (ObservationRecord, error) {
	var record ObservationRecord
	data, err := io.ReadAll(io.LimitReader(reader, maxRecordBytes+1))
	if err != nil {
		return record, err
	}
	if len(data) > maxRecordBytes {
		return record, fmt.Errorf("observation record exceeds 128 MiB")
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return record, err
	}
	return record, record.Validate()
}

func LoadRecord(path string) (ObservationRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return ObservationRecord{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return ObservationRecord{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxRecordBytes {
		return ObservationRecord{}, fmt.Errorf("expected a regular observation record file no larger than 128 MiB")
	}
	return DecodeRecord(f)
}

// SaveRecord creates a unique private file, never overwrites a user's file,
// and removes a partial file if writing or closing fails.
func SaveRecord(dir string, record ObservationRecord) (string, error) {
	if err := record.Validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	if len(data) >= maxRecordBytes {
		return "", fmt.Errorf("observation record exceeds 128 MiB")
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "c-observer-"+record.RecordedAt.Format("20060102-150405")+"-*.json")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err = f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}


// ValidateRun protects replay from malformed offsets, including non-monotonic ones.
func ValidateRun(run observer.RunResult) error {
	if run.Reason == "" {
		return fmt.Errorf("missing run result")
	}
	prev := observer.OutputOffsets{}
	for i, s := range run.Snapshots {
		o := s.OutputOffsets
		if s.ID != i || o.StdoutBytes < prev.StdoutBytes || o.StderrBytes < prev.StderrBytes ||
			o.StdoutBytes > len(run.Output.Stdout) || o.StderrBytes > len(run.Output.Stderr) {
			return fmt.Errorf("invalid snapshot or output offset at %d", i)
		}
		prev = o
	}
	return nil
}

