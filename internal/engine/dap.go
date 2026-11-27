package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

const maxDAPMessage = 8 << 20

type dapMessage struct {
	Type       string          `json:"type"`
	Event      string          `json:"event"`
	RequestSeq int             `json:"request_seq"`
	Success    *bool           `json:"success"`
	Message    string          `json:"message"`
	Body       json.RawMessage `json:"body"`
}

type dapWrite struct {
	data   []byte
	result chan error
}

type dapRead struct {
	message dapMessage
	err     error
}

type adapter struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	read        chan dapRead
	write       chan dapWrite
	done        chan struct{}
	exited      chan error
	seq         int
	events      []dapMessage
	stderr      *cappedBuffer
	console     cappedBuffer
	launchSeq   int
	launchReply *dapMessage
}

func openAdapter(path, dir string) (*adapter, error) {
	cmd := exec.Command(path)
	cmd.Dir = dir
	stderr := &cappedBuffer{limit: 8192}
	cmd.Stderr = stderr
	cmd.WaitDelay = cleanupTimeout / 2
	configureProcess(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}
	d := &adapter{cmd: cmd, stdin: stdin, stdout: stdout, read: make(chan dapRead),
		write: make(chan dapWrite), done: make(chan struct{}), exited: make(chan error, 1),
		stderr: stderr, console: cappedBuffer{limit: 8192}}
	go d.readMessages()
	go d.writeMessages()
	return d, nil
}

func (d *adapter) readMessages() {
	defer close(d.read)
	r := bufio.NewReader(d.stdout)
	for {
		msg, err := readDAP(r)
		select {
		case d.read <- dapRead{message: msg, err: err}:
		case <-d.done:
			return
		}
		if err != nil {
			return
		}
	}
}

func (d *adapter) writeMessages() {
	for {
		select {
		case request := <-d.write:
			_, err := fmt.Fprintf(d.stdin, "Content-Length: %d\r\n\r\n", len(request.data))
			if err == nil {
				_, err = d.stdin.Write(request.data)
			}
			request.result <- err
		case <-d.done:
			return
		}
	}
}

func readDAP(r *bufio.Reader) (dapMessage, error) {
	var message dapMessage
	size := 0
	for {
		line, err := r.ReadSlice('\n')
		if err != nil {
			return message, err
		}
		header := strings.TrimSpace(string(line))
		if header == "" {
			break
		}
		key, value, found := strings.Cut(header, ":")
		if found && strings.EqualFold(key, "Content-Length") {
			size, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return message, err
			}
		}
	}
	if size <= 0 || size > maxDAPMessage {
		return message, fmt.Errorf("invalid DAP message length %d", size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return message, err
	}
	return message, json.Unmarshal(data, &message)
}

func (d *adapter) receive(ctx context.Context) (dapMessage, error) {
	select {
	case m, ok := <-d.read:
		if !ok {
			return dapMessage{}, io.EOF
		}
		return m.message, m.err
	case <-ctx.Done():
		return dapMessage{}, ctx.Err()
	}
}

func (d *adapter) send(ctx context.Context, command string, args any) (int, error) {
	d.seq++
	seq := d.seq
	data, err := json.Marshal(map[string]any{"type": "request", "seq": seq, "command": command, "arguments": args})
	if err != nil {
		return 0, err
	}
	write := dapWrite{data: data, result: make(chan error, 1)}
	select {
	case d.write <- write:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	select {
	case err := <-write.result:
		if err != nil {
			return 0, err
		}
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	return seq, nil
}

func (d *adapter) request(ctx context.Context, command string, args any, body any) error {
	seq, err := d.send(ctx, command, args)
	if err != nil {
		return err
	}
	return d.waitReply(ctx, seq, body)
}

func (d *adapter) waitReply(ctx context.Context, seq int, body any) error {
	if seq == d.launchSeq && d.launchReply != nil {
		return responseError(*d.launchReply)
	}
	for {
		message, err := d.receive(ctx)
		if err != nil {
			return err
		}
		switch message.Type {
		case "event":
			if err := d.saveEvent(message); err != nil {
				return err
			}
		case "response":
			if message.RequestSeq != seq {
				if err := d.rememberLaunch(message); err != nil {
					return err
				}
				continue
			}
			if err := responseError(message); err != nil {
				return err
			}
			if body != nil {
				return json.Unmarshal(message.Body, body)
			}
			return nil
		default:
			return fmt.Errorf("unexpected DAP message type %q", message.Type)
		}
	}
}

// LLDB versions differ on whether launch replies before or after configurationDone.
func (d *adapter) rememberLaunch(message dapMessage) error {
	if d.launchSeq != 0 && message.Type == "response" && message.RequestSeq == d.launchSeq {
		d.launchReply = &message
		if err := responseError(message); err != nil {
			return fmt.Errorf("launch: %w", err)
		}
	}
	return nil
}

// A negative adapter response is distinct from transport failure/timeouts.
type dapRejectedError string

func (e dapRejectedError) Error() string { return string(e) }

func responseError(message dapMessage) error {
	if message.Success == nil {
		return errors.New("LLDB response omitted the success field")
	}
	if *message.Success {
		return nil
	}
	var details struct {
		Error struct {
			ID        int
			Format    string
			Variables map[string]string
		}
	}
	_ = json.Unmarshal(message.Body, &details)
	if description := strings.TrimSpace(details.Error.Format); description != "" {
		for key, value := range details.Error.Variables {
			description = strings.ReplaceAll(description, "{"+key+"}", value)
		}
		return dapRejectedError(description)
	}
	if description := strings.TrimSpace(message.Message); description != "" {
		return dapRejectedError(description)
	}
	if details.Error.ID != 0 {
		return dapRejectedError(fmt.Sprintf("LLDB rejected the request (error %d)", details.Error.ID))
	}
	return dapRejectedError("LLDB rejected the request without an error message")
}

func (d *adapter) saveEvent(message dapMessage) error {
	switch message.Event {
	case "output":
		var output struct{ Category, Output string }
		if json.Unmarshal(message.Body, &output) == nil {
			switch output.Category {
			case "console", "important", "stderr":
				_, _ = d.console.Write([]byte(output.Output))
			}
		}
	case "initialized", "stopped", "exited", "terminated":
		if len(d.events) == 64 {
			return errors.New("too many pending DAP events")
		}
		d.events = append(d.events, message)
	}
	return nil
}

func (d *adapter) event(ctx context.Context, names ...string) (dapMessage, error) {
	for {
		for i, message := range d.events {
			for _, name := range names {
				if message.Event == name {
					d.events = append(d.events[:i], d.events[i+1:]...)
					return message, nil
				}
			}
		}
		message, err := d.receive(ctx)
		if err != nil {
			return message, err
		}
		if message.Type == "event" {
			if err := d.saveEvent(message); err != nil {
				return message, err
			}
		} else if err := d.rememberLaunch(message); err != nil {
			return message, err
		}
	}
}
