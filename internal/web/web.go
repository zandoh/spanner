// Package web is the local UI: a single-page front end over the rig
// pipeline with live progress via server-sent events. It binds to loopback —
// this is a personal workbench, not a service.
package web

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/zandoh/spanner/internal/profile"
	"github.com/zandoh/spanner/internal/workshop"
)

//go:embed index.html
var indexHTML embed.FS

// Server wires the rig pipeline to HTTP. Runner is injected so tests (and
// future backends) can swap the sim execution.
type Server struct {
	Store  *workshop.Store
	OutDir string
	Runner Runner

	mu   sync.Mutex
	runs map[string]*run
	// oneSim serializes sims: they saturate every core, so concurrent runs
	// only slow each other down.
	oneSim chan struct{}
}

// Runner executes a prepared sim input, streaming console output to
// progress, and returns the report's filename within OutDir. The CLI wires
// this to the rig pipeline.
type Runner interface {
	Run(progress io.Writer, inputPath, stem, display string, iterations int, targetError float64, weights bool) (string, error)
}

// New builds a Server; runner must not be nil.
func New(store *workshop.Store, outDir string, runner Runner) *Server {
	return &Server{
		Store:  store,
		OutDir: outDir,
		Runner: runner,
		runs:   make(map[string]*run),
		oneSim: make(chan struct{}, 1),
	}
}

// Handler returns the full route table.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /{$}", http.FileServer(http.FS(indexHTML)))
	mux.HandleFunc("GET /api/characters", s.handleCharacters)
	mux.HandleFunc("POST /api/sim", s.handleSim)
	mux.HandleFunc("GET /api/runs/{id}/events", s.handleEvents)
	mux.Handle("GET /reports/", http.StripPrefix("/reports/", http.FileServer(http.Dir(s.OutDir))))
	return mux
}

func (s *Server) handleCharacters(w http.ResponseWriter, r *http.Request) {
	entries, err := s.Store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, entries)
}

type simRequest struct {
	Import      string  `json:"import"`
	Char        string  `json:"char"`
	Iterations  int     `json:"iterations"`
	TargetError float64 `json:"targetError"`
	Weights     bool    `json:"weights"`
}

func (s *Server) handleSim(w http.ResponseWriter, r *http.Request) {
	var req simRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
		http.Error(w, "bad request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	var prof *profile.Profile
	var err error
	switch {
	case req.Char != "" && req.Import != "":
		http.Error(w, "send either import or char, not both", http.StatusBadRequest)
		return
	case req.Char != "":
		prof, err = s.Store.Load(req.Char)
	case req.Import != "":
		prof, err = profile.ParseExport(strings.NewReader(req.Import))
	default:
		http.Error(w, "import text or char name required", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	tmp, err := os.CreateTemp("", "spanner-web-*.simc")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, werr := tmp.WriteString(prof.Raw)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp.Name())
		http.Error(w, werr.Error(), http.StatusInternalServerError)
		return
	}

	id := newID()
	ru := newRun()
	s.mu.Lock()
	s.runs[id] = ru
	s.mu.Unlock()

	display := prof.Name
	if prof.Spec != "" {
		display = fmt.Sprintf("%s (%s %s)", prof.Name, prof.Spec, prof.Class)
	}
	go func() {
		defer func() { _ = os.Remove(tmp.Name()) }()
		select {
		case s.oneSim <- struct{}{}:
		default:
			ru.line("queued behind another sim...")
			s.oneSim <- struct{}{}
		}
		defer func() { <-s.oneSim }()

		report, err := s.Runner.Run(ru, tmp.Name(), prof.Slug(), display, req.Iterations, req.TargetError, req.Weights)
		if err != nil {
			ru.finish("", err)
			return
		}
		ru.finish("/reports/"+report, nil)
	}()

	writeJSON(w, map[string]string{"id": id})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	ru := s.runs[r.PathValue("id")]
	s.mu.Unlock()
	if ru == nil {
		http.Error(w, "unknown run", http.StatusNotFound)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ch, replay := ru.subscribe()
	defer ru.unsubscribe(ch)
	for _, ev := range replay {
		_, _ = fmt.Fprint(w, ev)
	}
	flusher.Flush()
	for {
		select {
		case ev, open := <-ch:
			if !open {
				return
			}
			_, _ = fmt.Fprint(w, ev)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
