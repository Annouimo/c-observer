package engine

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Annouimo/c-observer/internal/observer"
)

//go:embed gdb_capture.py
var gdbCapture []byte

type miLine struct {
	text string
	err  error
}

type gdbMI struct {
	running bool
	cmd     *exec.Cmd
	in      io.WriteCloser
	out     io.ReadCloser
	lines   chan miLine
	done    chan struct{}
	stops   []string
	seq     int
	console string
	stderr  *cappedBuffer
}

func startMI(path, dir string) (*gdbMI, error) {
	cmd := exec.Command(path, "-nx", "-q", "--interpreter=mi2")
	cmd.Dir = dir
	configureProcess(cmd)
	stderr := &cappedBuffer{limit: 8192}
	cmd.Stderr = stderr
	cmd.WaitDelay = cleanupTimeout / 2
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		in.Close()
		out.Close()
		return nil, err
	}
	m := &gdbMI{cmd: cmd, in: in, out: out, lines: make(chan miLine), done: make(chan struct{}), stderr: stderr}
	go func() {
		defer close(m.lines)
		r := bufio.NewScanner(out)
		r.Buffer(make([]byte, 4096), maxDAPMessage)
		for r.Scan() {
			select {
			case m.lines <- miLine{text: r.Text()}:
			case <-m.done:
				return
			}
		}
		if err := r.Err(); err != nil {
			select {
			case m.lines <- miLine{err: err}:
			case <-m.done:
			}
		}
	}()
	return m, nil
}

func (m *gdbMI) read(ctx context.Context) (string, error) {
	select {
	case l, ok := <-m.lines:
		if !ok {
			return "", io.EOF
		}
		return l.text, l.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (m *gdbMI) request(ctx context.Context, command string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.console = ""
	m.seq++
	token := strconv.Itoa(m.seq)
	if deadline, ok := ctx.Deadline(); ok {
		if pipe, ok := m.in.(*os.File); ok {
			_ = pipe.SetWriteDeadline(deadline)
		}
	}
	if _, err := fmt.Fprintln(m.in, token+command); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return context.DeadlineExceeded
		}
		return err
	}
	for {
		line, err := m.read(ctx)
		if err != nil {
			return err
		}
		switch {
		case strings.HasPrefix(line, "*stopped"):
			m.running = false
			m.stops = append(m.stops, line)
		case strings.HasPrefix(line, `~"`) || strings.HasPrefix(line, `&"`):
			if decoded, err := strconv.Unquote(line[1:]); err == nil && len(m.console)+len(decoded) <= maxDAPMessage {
				m.console += decoded
			}
		case strings.HasPrefix(line, token+"^error"):
			return fmt.Errorf("%s: %s", strings.SplitN(command, " ", 2)[0], strings.TrimSpace(m.console+" "+line))
		case strings.HasPrefix(line, token+"^"):
			if strings.HasPrefix(line, token+"^running") {
				m.running = true
			}
			return nil
		}
	}
}

func (m *gdbMI) stop(ctx context.Context) (string, error) {
	if len(m.stops) > 0 {
		line := m.stops[0]
		m.stops = m.stops[1:]
		return line, nil
	}
	for {
		line, err := m.read(ctx)
		if strings.HasPrefix(line, "*stopped") {
			m.running = false
			return line, nil
		}
		if err != nil {
			return line, err
		}
	}
}

func (m *gdbMI) close() string {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout/2)
	defer cancel()
	// An interrupted request can still reply. Finish the inferior before exiting GDB.
	if m.running && m.request(ctx, "-exec-interrupt") == nil {
		_, _ = m.stop(ctx)
	}
	_ = m.request(ctx, miConsole("kill"))
	err := m.request(ctx, "-gdb-exit")
	close(m.done)
	m.in.Close()
	m.out.Close()
	wait := make(chan error, 1)
	go func() { wait <- m.cmd.Wait() }()
	select {
	case waitErr := <-wait:
		if waitErr != nil {
			err = errors.Join(err, waitErr)
		}
	case <-ctx.Done():
		killErr := killProcessGroup(m.cmd)
		select {
		case <-wait:
		case <-time.After(cleanupTimeout / 2):
		}
		return fmt.Sprintf("GDB graceful cleanup did not complete: %v", errors.Join(ctx.Err(), err, killErr))
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func miConsole(command string) string { return "-interpreter-exec console " + strconv.Quote(command) }
func shellPath(path string) string    { return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'" }

type gdbOptions struct {
	Source              string
	Files               []string
	Ranges              []observer.ArrayRange
	Depth, Nodes, Array int
	Map                 map[string]string
	Directory           string
}

func (c *capture) observeGDB(debugger, exe, input, temp, source string) error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	script := filepath.Join(temp, "observe.py")
	if err := os.WriteFile(script, gdbCapture, 0600); err != nil {
		return err
	}
	m, err := startMI(debugger, c.dir)
	if err != nil {
		return err
	}
	c.result.Session.Backend = "gdb-mi"
	defer func() {
		c.result.CleanupError = m.close()
		state, err := readOutput(c.output, c.stderr, c.limits.MaxOutputBytes)
		c.result.Output = state
		if err != nil {
			c.result.Diagnostics = append(c.result.Diagnostics, observer.Diagnostic{Severity: "error", Message: err.Error()})
		}
		if msg := strings.TrimSpace(m.stderr.String()); msg != "" {
			c.result.Diagnostics = append(c.result.Diagnostics, observer.Diagnostic{Severity: "warning", Message: msg})
		}
	}()
	request := func(command string) error {
		ctx, cancel := context.WithTimeout(c.ctx, time.Duration(c.limits.RequestTimeoutMillis)*time.Millisecond)
		defer cancel()
		return m.request(ctx, command)
	}
	quotedJSON := func(value any) string {
		data, _ := json.Marshal(value)
		return string(data)
	}
	opts := gdbOptions{Source: source, Ranges: c.config.ArrayRanges,
		Depth: c.limits.MaxDepth, Nodes: c.limits.MaxSnapshotNodes, Array: c.limits.MaxArrayElements,
		Map: c.config.SourceMap, Directory: c.dir}
	for _, file := range c.result.Session.Sources {
		if !c.skip(file.Path) {
			opts.Files = append(opts.Files, file.Path)
		}
	}
	init := "python exec(compile(open(" + quotedJSON(script) + ", encoding='utf-8').read(), " + quotedJSON(script) + ", 'exec')); observer_init(" + quotedJSON(quotedJSON(opts)) + ")"
	for _, cmd := range []string{"-gdb-set confirm off", "-gdb-set disable-randomization off",
		"-gdb-set mi-async on", "-gdb-set backtrace past-main on", "-gdb-set auto-load off", "-gdb-set debuginfod enabled off", "-gdb-set print elements 16",
		"-file-exec-and-symbols " + strconv.Quote(exe), miConsole(init)} {
		if err := request(cmd); err != nil {
			return err
		}
	}
	for _, loc := range c.config.KeepLocations {
		file := loc.ReportedFile
		if file == "" {
			file = absolute(c.dir, loc.File)
		}
		if err := request("-break-insert --source " + strconv.Quote(file) + " --line " + strconv.Itoa(loc.Line)); err != nil {
			c.result.Diagnostics = append(c.result.Diagnostics, observer.Diagnostic{Severity: "warning", Message: err.Error()})
		}
	}
	c.result.Session.Capabilities.ExplicitStops = true
	if err := request("-exec-arguments < " + shellPath(input) + " > " + shellPath(c.output) + " 2> " + shellPath(c.stderr)); err != nil {
		return err
	}
	if err := request("-exec-run"); err != nil {
		return err
	}
	var history int64
	for {
		stopCtx, cancel := context.WithTimeout(c.ctx, time.Duration(c.limits.RequestTimeoutMillis)*time.Millisecond)
		line, err := m.stop(stopCtx)
		cancel()
		if err != nil {
			return err
		}
		switch {
		case strings.Contains(line, `reason="exited-normally"`):
			code := 0
			c.result.ExitCode, c.result.Reason = &code, observer.Exited
			return nil
		case strings.Contains(line, `reason="exited"`):
			_, tail, _ := strings.Cut(line, `exit-code="`)
			number, _, _ := strings.Cut(tail, `"`)
			code, err := strconv.ParseInt(number, 0, 32)
			if err != nil {
				return fmt.Errorf("could not read GDB exit code: %s", line)
			}
			exitCode := int(code)
			c.result.ExitCode, c.result.Reason = &exitCode, observer.NonzeroExit
			return nil
		}
		crashed := strings.Contains(line, `reason="signal-received"`) || strings.Contains(line, `reason="exited-signalled"`)
		if err := request(miConsole("python observer_capture()")); err != nil {
			if crashed {
				c.result.Reason, c.result.Message = observer.Signaled, line
				return nil
			}
			return err
		}
		_, payload, found := strings.Cut(m.console, "COBS:")
		if !found {
			return errors.New("GDB did not return an observation snapshot")
		}
		payload, _, _ = strings.Cut(payload, "\n")
		var snapshot observer.Snapshot
		if err := json.Unmarshal([]byte(payload), &snapshot); err != nil {
			return fmt.Errorf("decode GDB observation: %w", err)
		}
		visible := snapshot.Location.File != "" && !c.skip(snapshot.Location.File)
		command := "-exec-continue"
		if visible || crashed || c.explicit(snapshot.Location) {
			snapshot.ID = len(c.result.Snapshots)
			snapshot.OutputOffsets.StdoutBytes, err = outputSize(c.output, c.limits.MaxOutputBytes)
			if err != nil {
				return err
			}
			snapshot.OutputOffsets.StderrBytes, err = outputSize(c.stderr, c.limits.MaxOutputBytes)
			if err != nil {
				return err
			}
			history += int64(len(payload))
			if history > c.limits.MaxHistoryBytes {
				c.result.Reason, c.result.Message = observer.LimitReached, "history byte limit reached"
				return nil
			}
			c.result.Snapshots = append(c.result.Snapshots, snapshot)
			c.result.Session.Capabilities.Globals = c.result.Session.Capabilities.Globals || len(snapshot.Globals) > 0
			if visible {
				command = "-exec-next"
			}
		} else {
			for _, frame := range snapshot.Frames {
				if frame.Location.File != "" && !c.skip(frame.Location.File) {
					command = "-exec-finish"
					break
				}
			}
		}
		if crashed {
			c.result.Reason, c.result.Message = observer.Signaled, line
			return nil
		}
		if len(c.result.Snapshots) >= c.limits.MaxSnapshots {
			c.result.Reason, c.result.Message = observer.LimitReached, "snapshot limit reached"
			return nil
		}
		if err := request(command); err != nil {
			return err
		}
	}
}
