package forge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestUpdateEndToEnd exercises the full fetch pipeline against a local
// server, with a real (tiny) disk image so extraction runs for real.
// darwin-only: extraction is hdiutil-based.
func TestUpdateEndToEnd(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("dmg extraction is darwin-only")
	}
	if _, err := exec.LookPath("hdiutil"); err != nil {
		t.Skip("hdiutil unavailable")
	}

	// A disk image holding a stand-in simc and the GPL notice.
	vol := t.TempDir()
	fakeSimc := "#!/bin/sh\necho fake simc\n"
	if err := os.WriteFile(filepath.Join(vol, "simc"), []byte(fakeSimc), 0o700); err != nil { // #nosec G306 -- stand-in binary
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vol, "COPYING"), []byte("GPL-3.0"), 0o600); err != nil {
		t.Fatal(err)
	}
	dmg := filepath.Join(t.TempDir(), "simc-1205-01-macos-e35c129.dmg")
	if out, err := exec.Command("hdiutil", "create", "-quiet", "-srcfolder", vol, "-volname", "simc", "-format", "UDZO", dmg).CombinedOutput(); err != nil {
		t.Fatalf("hdiutil create: %v: %s", err, out)
	}

	var downloads int
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<a href="simc-1205-01-macos-e35c129.dmg">simc-1205-01-macos-e35c129.dmg</a>`)
	})
	mux.HandleFunc("/simc-1205-01-macos-e35c129.dmg", func(w http.ResponseWriter, r *http.Request) {
		downloads++
		http.ServeFile(w, r, dmg)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cache := t.TempDir()
	f := &Fetcher{BaseURL: srv.URL + "/", CacheDir: cache}

	got, err := f.Update(context.Background())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	body, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != fakeSimc {
		t.Error("installed binary does not match the image's simc")
	}

	installDir := filepath.Dir(got)
	if _, err := os.Stat(filepath.Join(installDir, "COPYING")); err != nil {
		t.Error("GPL notice was not copied alongside the binary")
	}
	raw, err := os.ReadFile(filepath.Join(installDir, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest json: %v", err)
	}
	if m.Version != "1205-01" || m.Commit != "e35c129" || len(m.SHA256) != 64 {
		t.Errorf("manifest = %+v", m)
	}

	// Second update must reuse the install, not re-download.
	again, err := f.Update(context.Background())
	if err != nil {
		t.Fatalf("second Update: %v", err)
	}
	if again != got {
		t.Errorf("second Update returned %q, want %q", again, got)
	}
	if downloads != 1 {
		t.Errorf("downloads = %d, want 1", downloads)
	}

	// And the cache-locate path must now find it.
	newest, ok := newestInstalled(cache)
	if !ok || newest != got {
		t.Errorf("newestInstalled = (%q, %v), want %q", newest, ok, got)
	}
}

func TestUpdateEmptyIndex(t *testing.T) {
	if platformOS() == "" {
		t.Skip("no upstream builds for this platform")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "<html>no builds here</html>")
	}))
	defer srv.Close()

	f := &Fetcher{BaseURL: srv.URL + "/", CacheDir: t.TempDir()}
	_, err := f.Update(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no") {
		t.Errorf("want no-builds error, got %v", err)
	}
}
