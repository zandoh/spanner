package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// run is one sim's event log and its live subscribers. It implements
// io.Writer so it can sit directly under the rig pipeline's progress stream,
// splitting console output into SSE events.
type run struct {
	mu     sync.Mutex
	events []string // formatted SSE frames, replayed to late subscribers
	subs   map[chan string]bool
	done   bool
	buf    bytes.Buffer // partial console line carried between writes
}

func newRun() *run {
	return &run{subs: make(map[chan string]bool)}
}

// Write splits simc console output into per-line progress events.
func (r *run) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf.Write(p)
	for {
		line, err := r.buf.ReadString('\n')
		if err != nil {
			// Partial line: keep it for the next write. SimC's progress
			// ticker rewrites one \r line, so split on \r too.
			r.buf.WriteString(line)
			break
		}
		for _, part := range strings.Split(strings.TrimRight(line, "\n"), "\r") {
			if part = strings.TrimSpace(part); part != "" {
				r.emit("progress", part)
			}
		}
	}
	return len(p), nil
}

func (r *run) line(msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.emit("progress", msg)
}

// finish emits the terminal event: done with a report URL, or error.
func (r *run) finish(reportURL string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.emit("error", err.Error())
	} else {
		r.emit("done", reportURL)
	}
	r.done = true
	for ch := range r.subs {
		close(ch)
		delete(r.subs, ch)
	}
}

// emit formats one SSE frame; callers hold r.mu.
func (r *run) emit(event, data string) {
	payload, marshalErr := json.Marshal(data)
	if marshalErr != nil {
		return
	}
	frame := fmt.Sprintf("event: %s\ndata: %s\n\n", event, payload)
	r.events = append(r.events, frame)
	for ch := range r.subs {
		select {
		case ch <- frame:
		default: // a stalled subscriber must not block the sim
		}
	}
}

// subscribe returns a live channel plus every frame so far. The channel is
// already closed for finished runs — replay carries the whole story.
func (r *run) subscribe() (chan string, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan string, 64)
	replay := append([]string(nil), r.events...)
	if r.done {
		close(ch)
	} else {
		r.subs[ch] = true
	}
	return ch, replay
}

func (r *run) unsubscribe(ch chan string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.subs[ch] {
		delete(r.subs, ch)
		close(ch)
	}
}
