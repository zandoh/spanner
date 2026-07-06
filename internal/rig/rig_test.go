package rig

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zandoh/spanner/internal/crank"
)

// fakeSimc copies the gauge fixture to the requested json2 path, so the
// full pipeline runs without a real SimC binary.
func fakeSimc(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake simc is a shell script")
	}
	fixture, err := filepath.Abs("../gauge/testdata/midnight-1205.json")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "simc")
	script := "#!/bin/sh\nfor a in \"$@\"; do case \"$a\" in json2=*) cp " + fixture + " \"${a#json2=}\";; esac; done\necho simulating\n"
	if err := os.WriteFile(p, []byte(script), 0o700); err != nil { // #nosec G306 -- stand-in binary
		t.Fatal(err)
	}
	return p
}

func TestRunPipeline(t *testing.T) {
	outDir := t.TempDir()
	var progress bytes.Buffer

	res, err := Run(context.Background(), Job{
		SimcPath:  fakeSimc(t),
		InputPath: "whatever.simc",
		Stem:      "zandy",
		Display:   "Zandy (blood deathknight)",
		Options:   crank.Options{Iterations: 100},
		OutDir:    outDir,
		Progress:  &progress,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.Report.Sim.Players[0].CollectedData.DPS.Mean <= 0 {
		t.Error("parsed report has no dps")
	}
	html, err := os.ReadFile(res.HTMLPath)
	if err != nil {
		t.Fatalf("report not written: %v", err)
	}
	if !strings.Contains(string(html), "Damage per second") {
		t.Error("report content missing")
	}
	if !strings.HasPrefix(filepath.Base(res.HTMLPath), "zandy-") {
		t.Errorf("stem not applied: %s", res.HTMLPath)
	}
	if !strings.Contains(progress.String(), "cranking Zandy") {
		t.Error("progress line missing")
	}
}

func TestRunNilProgressAndBadBinary(t *testing.T) {
	_, err := Run(context.Background(), Job{
		SimcPath:  filepath.Join(t.TempDir(), "missing"),
		InputPath: "x.simc",
		Stem:      "x",
		OutDir:    t.TempDir(),
	})
	if err == nil {
		t.Error("want error for missing binary")
	}
}
