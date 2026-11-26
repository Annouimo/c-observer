package engine

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Routilet/c-observer/internal/observer"
	"testing"
)

func TestDAPResponseError(t *testing.T) {
	for _, tc := range []struct {
		packet string
		want   string
	}{
		{`{"success":true}`, ""},
		{`{"success":false}`, "LLDB rejected the request without an error message"},
		{`{"message":"unexpected"}`, "LLDB response omitted the success field"},
		{`{"success":false,"message":"direct diagnostic"}`, "direct diagnostic"},
		{`{"success":false,"message":"invalidArguments","body":{"error":{"id":9,"format":"Invalid {argument}","variables":{"argument":"pathFormat"}}}}`, "Invalid pathFormat"},
		{`{"success":false,"body":{"error":{"id":9}}}`, "LLDB rejected the request (error 9)"},
	} {
		t.Run(tc.packet, func(t *testing.T) {
			var message dapMessage
			if err := json.Unmarshal([]byte(tc.packet), &message); err != nil {
				t.Fatal(err)
			}
			err := responseError(message)
			if tc.want == "" && err == nil {
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLaunchReplyOrdering(t *testing.T) {
	for _, test := range []struct {
		name     string
		deferred bool
	}{
		{"reply before initialized", false},
		{"reply after configurationDone", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			d := &adapter{read: make(chan dapRead, 8), write: make(chan dapWrite)}
			go func() {
				success := true
				reply := func(seq int, body string) {
					d.read <- dapRead{message: dapMessage{Type: "response", RequestSeq: seq, Success: &success, Body: json.RawMessage(body)}}
				}
				event := func(name, body string) {
					d.read <- dapRead{message: dapMessage{Type: "event", Event: name, Body: json.RawMessage(body)}}
				}
				for {
					select {
					case <-ctx.Done():
						return
					case write := <-d.write:
						var req struct {
							Seq     int
							Command string
						}
						err := json.Unmarshal(write.data, &req)
						write.result <- err
						switch req.Command {
						case "initialize":
							reply(req.Seq, `{"supportsFunctionBreakpoints":true}`)
						case "launch":
							if !test.deferred {
								reply(req.Seq, "{}")
							}
							event("initialized", "{}")
						case "setFunctionBreakpoints":
							reply(req.Seq, "{}")
						case "configurationDone":
							if test.deferred {
								reply(2, "{}")
							}
							event("exited", `{"exitCode":0}`)
							reply(req.Seq, "{}")
							return
						}
					}
				}
			}()
			result := observer.RunResult{}
			run := capture{adapter: d, ctx: ctx, limits: observer.Limits{RequestTimeoutMillis: 500}, result: &result}
			if err := run.observe("program", "input"); err != nil || result.Reason != observer.Exited {
				t.Fatalf("observe: %v, result: %+v", err, result)
			}
		})
	}
}
