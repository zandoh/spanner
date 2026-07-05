package forge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocatePrecedence(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "simc-explicit")
	fromEnv := filepath.Join(dir, "simc-env")
	for _, p := range []string{explicit, fromEnv} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o700); err != nil { // #nosec G306 -- stand-in binary must be executable
			t.Fatal(err)
		}
	}

	cache := t.TempDir()
	cached := fakeInstall(t, cache, "1205-01-e35c129")

	t.Setenv(EnvVar, fromEnv)
	got, err := Locate(explicit, cache)
	if err != nil || got != explicit {
		t.Errorf("explicit path: got (%q, %v), want %q", got, err, explicit)
	}

	got, err = Locate("", cache)
	if err != nil || got != fromEnv {
		t.Errorf("env fallback: got (%q, %v), want %q", got, err, fromEnv)
	}

	t.Setenv(EnvVar, "")
	got, err = Locate("", cache)
	if err != nil || got != cached {
		t.Errorf("cache fallback: got (%q, %v), want %q", got, err, cached)
	}
}

func TestLocateErrors(t *testing.T) {
	if _, err := Locate(filepath.Join(t.TempDir(), "missing"), ""); err == nil {
		t.Error("want error for nonexistent explicit path")
	}
	if _, err := Locate(t.TempDir(), ""); err == nil {
		t.Error("want error when path is a directory")
	}
}
