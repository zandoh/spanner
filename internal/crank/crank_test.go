package crank

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeSimc stands in for the real binary: a script that reports the
// arguments crank assembled, or misbehaves on demand.
func fakeSimc(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake simc is a shell script")
	}
	p := filepath.Join(t.TempDir(), "simc")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil { // #nosec G306 -- stand-in binary must be executable
		t.Fatal(err)
	}
	return p
}

func TestRunAssemblesArguments(t *testing.T) {
	sim := fakeSimc(t, `echo "$@"`)
	var progress bytes.Buffer

	opts := Options{Iterations: 500, Threads: 4, TargetError: 0.5}
	if err := Run(context.Background(), sim, "char.simc", "out.json", opts, &progress); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := strings.TrimSpace(progress.String())
	want := "char.simc json2=out.json iterations=500 threads=4 target_error=0.5"
	if got != want {
		t.Errorf("simc argv:\n got %q\nwant %q", got, want)
	}
}

func TestRunOmitsZeroOptions(t *testing.T) {
	sim := fakeSimc(t, `echo "$@"`)
	var progress bytes.Buffer

	if err := Run(context.Background(), sim, "char.simc", "out.json", Options{}, &progress); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := strings.TrimSpace(progress.String())
	if got != "char.simc json2=out.json" {
		t.Errorf("zero options must defer to simc defaults, got argv %q", got)
	}
}

func TestRunStreamsStderrToo(t *testing.T) {
	sim := fakeSimc(t, `echo "progress tick" 1>&2`)
	var progress bytes.Buffer

	if err := Run(context.Background(), sim, "p.simc", "o.json", Options{}, &progress); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(progress.String(), "progress tick") {
		t.Error("stderr output did not reach the progress writer")
	}
}

func TestRunReportsFailure(t *testing.T) {
	sim := fakeSimc(t, `echo "Nothing to sim!"; exit 3`)
	var progress bytes.Buffer

	err := Run(context.Background(), sim, "p.simc", "o.json", Options{}, &progress)
	if err == nil || !strings.Contains(err.Error(), "simc run failed") {
		t.Errorf("want wrapped failure, got %v", err)
	}
	if !strings.Contains(progress.String(), "Nothing to sim!") {
		t.Error("output preceding the failure was not streamed")
	}
}

func TestRunKillsOnContextCancel(t *testing.T) {
	sim := fakeSimc(t, `sleep 30`)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := Run(ctx, sim, "p.simc", "o.json", Options{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("want error after context timeout")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("process not killed promptly: took %v", elapsed)
	}
}
