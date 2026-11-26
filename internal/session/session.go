// Package session defines the versioned external launch file, independent of the engine API.
package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Routilet/c-observer/internal/observer"
)

const Version = 1
const maxConfig = 1 << 20
const maxSample = 4 << 20

type File struct {
	SchemaVersion int               `json:"schemaVersion"`
	SourcePath string               `json:"sourcePath"`
	WorkDir string                  `json:"workDir"`
	VisibleSourcePaths []string     `json:"visibleSourcePaths,omitempty"`
	SkipSourcePaths []string        `json:"skipSourcePaths,omitempty"`
	SourceMap map[string]string     `json:"sourceMap,omitempty"`
	Compiler string                 `json:"compiler,omitempty"`
	Debugger string                 `json:"debugger,omitempty"`
	CompilerArgs []string           `json:"compilerArgs,omitempty"`
	StdinFile string                `json:"stdinFile"`
	ExpectedStdoutFile *string      `json:"expectedStdoutFile,omitempty"`
}

func readRegular(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil { return nil, err }
	if !info.Mode().IsRegular() || info.Size() > limit { return nil, fmt.Errorf("not a regular file or exceeds size limit: %s", path) }
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil { return nil, err }
	if int64(len(data)) > limit { return nil, fmt.Errorf("file exceeds size limit: %s", path) }
	return data, nil
}

// Load resolves session-owned paths relative to the JSON file, not WorkDir.
// SourceMap keys are debugger-reported logical paths and are preserved verbatim.
func Load(path string) (observer.LaunchConfig, error) {
	var cfg observer.LaunchConfig
	path, err := filepath.Abs(path)
	if err != nil { return cfg, err }
	data, err := readRegular(path,maxConfig)
	if err != nil { return cfg, err }
	var f File
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&f); err != nil { return cfg, err }
	var extra any
	if err := d.Decode(&extra); err != io.EOF { return cfg, fmt.Errorf("session must contain exactly one JSON object") }
	if f.SchemaVersion != Version { return cfg, fmt.Errorf("unsupported session schema: %d (supported: %d)", f.SchemaVersion, Version) }
	resolve := func(p string) string {
		if p == "" { return "" }
		if filepath.IsAbs(p) { return filepath.Clean(p) }
		return filepath.Join(filepath.Dir(path),p)
	}
	cfg.SourcePath = resolve(f.SourcePath)
	cfg.WorkDir = resolve(f.WorkDir)
	if cfg.WorkDir == "" { cfg.WorkDir = filepath.Dir(cfg.SourcePath) }
	if cfg.SourcePath == "" || strings.ToLower(filepath.Ext(cfg.SourcePath)) != ".c" || f.StdinFile == "" { return cfg, fmt.Errorf("sourcePath (.c) and stdinFile are required") }
	info, err := os.Stat(cfg.SourcePath)
	if err != nil || !info.Mode().IsRegular() { return cfg, fmt.Errorf("C source file unavailable: %s",cfg.SourcePath) }
	info, err = os.Stat(cfg.WorkDir)
	if err != nil || !info.IsDir() { return cfg, fmt.Errorf("working directory unavailable: %s",cfg.WorkDir) }
	visible := map[string]bool{cfg.SourcePath:true}
	for _, p := range f.VisibleSourcePaths {
		p = resolve(p)
		if p == "" { return cfg, fmt.Errorf("empty visible source path") }
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() { return cfg, fmt.Errorf("visible source unavailable: %s",p) }
		cfg.VisibleSourcePaths = append(cfg.VisibleSourcePaths,p)
		visible[p] = true
	}
	cfg.SourceMap = map[string]string{}
	for key, target := range f.SourceMap {
		target = resolve(target)
		if key == "" || !visible[target] { return cfg, fmt.Errorf("sourceMap target must be a visible source: %q",key) }
		cfg.SourceMap[key] = target
	}
	for _, p := range f.SkipSourcePaths {
		if p == "" { return cfg, fmt.Errorf("empty skip source path") }
		cfg.SkipSourcePaths = append(cfg.SkipSourcePaths,resolve(p))
	}
	// Tool names resolve via PATH; path overrides must be absolute.
	for _, tool := range []string{f.Compiler,f.Debugger} {
		if strings.ContainsAny(tool,"/\\") && !filepath.IsAbs(tool) { return cfg, fmt.Errorf("tool path must be absolute: %s",tool) }
	}
	cfg.Compiler, cfg.Debugger = f.Compiler,f.Debugger
	cfg.CompilerArgs = append([]string(nil), f.CompilerArgs...)
	cfg.Stdin, err = readRegular(resolve(f.StdinFile),maxSample)
	if err != nil { return cfg, fmt.Errorf("stdin: %w",err) }
	if f.ExpectedStdoutFile != nil {
		expected, err := readRegular(resolve(*f.ExpectedStdoutFile),maxSample)
		if err != nil { return cfg, fmt.Errorf("expected stdout: %w",err) }
		cfg.ExpectedStdout = &expected
	}
	return cfg,nil
}
