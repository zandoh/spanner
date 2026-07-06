package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zandoh/spanner/internal/profile"
	"github.com/zandoh/spanner/internal/workshop"
)

const export = "deathknight=\"Zandy\"\nspec=blood\nlevel=90\n"

// fakeRunner plays the rig: emits progress, writes a report file.
type fakeRunner struct {
	outDir string
	fail   error
}

func (f fakeRunner) Run(progress io.Writer, inputPath, stem, display string, iterations int, targetError float64, weights bool) (string, error) {
	// The input temp file must hold the export verbatim.
	raw, err := os.ReadFile(inputPath) // #nosec G304 -- test reads its own temp input
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(raw), "deathknight=") {
		return "", errors.New("input file missing profile")
	}
	_, _ = fmt.Fprintf(progress, "simulating %s (%d iterations)\n", display, iterations)
	if f.fail != nil {
		return "", f.fail
	}
	name := stem + "-test.html"
	return name, os.WriteFile(filepath.Join(f.outDir, name), []byte("<html>report</html>"), 0o600)
}

func newTestServer(t *testing.T, fail error) *httptest.Server {
	t.Helper()
	outDir := t.TempDir()
	store := &workshop.Store{Dir: t.TempDir()}
	p, err := profile.ParseExport(strings.NewReader(export))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("zandy", p); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(store, outDir, fakeRunner{outDir: outDir, fail: fail}).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func startSim(t *testing.T, srv *httptest.Server, body string) string {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/sim", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /api/sim: %d: %s", resp.StatusCode, raw)
	}
	var d struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatal(err)
	}
	return d.ID
}

// readEvents drains the SSE stream until it closes, returning the raw text.
func readEvents(t *testing.T, srv *httptest.Server, id string) string {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/runs/" + id + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSimFromImportToReport(t *testing.T) {
	srv := newTestServer(t, nil)
	id := startSim(t, srv, `{"import":"deathknight=\"Zandy\"\nspec=blood\nlevel=90\n","iterations":1000}`)

	events := readEvents(t, srv, id)
	if !strings.Contains(events, "event: progress") || !strings.Contains(events, "1000 iterations") {
		t.Errorf("progress events missing:\n%s", events)
	}
	if !strings.Contains(events, "event: done") {
		t.Fatalf("no done event:\n%s", events)
	}

	// The done event carries the report URL; fetch it.
	var url string
	for _, l := range strings.Split(events, "\n") {
		if strings.HasPrefix(l, "data: ") && strings.Contains(l, "/reports/") {
			_ = json.Unmarshal([]byte(strings.TrimPrefix(l, "data: ")), &url)
		}
	}
	if url == "" {
		t.Fatal("no report URL in done event")
	}
	resp, err := http.Get(srv.URL + url)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("report fetch: %v / %v", err, resp.Status)
	}
	_ = resp.Body.Close()

	// Late subscribers replay the full story.
	if replay := readEvents(t, srv, id); !strings.Contains(replay, "event: done") {
		t.Error("replay for finished run missing done event")
	}
}

func TestSimFromSavedCharacter(t *testing.T) {
	srv := newTestServer(t, nil)
	id := startSim(t, srv, `{"char":"zandy","iterations":500}`)
	if events := readEvents(t, srv, id); !strings.Contains(events, "Zandy (blood deathknight)") {
		t.Errorf("saved character not resolved:\n%s", events)
	}
}

func TestSimFailureSurfacesError(t *testing.T) {
	srv := newTestServer(t, errors.New("simc exploded"))
	id := startSim(t, srv, `{"char":"zandy"}`)
	events := readEvents(t, srv, id)
	if !strings.Contains(events, "event: error") || !strings.Contains(events, "simc exploded") {
		t.Errorf("error event missing:\n%s", events)
	}
}

func TestSimRejectsBadInput(t *testing.T) {
	srv := newTestServer(t, nil)
	for body, want := range map[string]int{
		`{}`:                         http.StatusBadRequest,
		`{"import":"not a profile"}`: http.StatusUnprocessableEntity,
		`{"char":"nobody"}`:          http.StatusUnprocessableEntity,
		`{"char":"a","import":"b"}`:  http.StatusBadRequest,
	} {
		resp, err := http.Post(srv.URL+"/api/sim", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("%s: status %d, want %d", body, resp.StatusCode, want)
		}
	}
}

func TestCharactersEndpoint(t *testing.T) {
	srv := newTestServer(t, nil)
	resp, err := http.Get(srv.URL + "/api/characters")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(raw), `"zandy"`) {
		t.Errorf("characters list missing save: %s", raw)
	}
}
