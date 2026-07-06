package forge

import (
	"os"
	"path/filepath"
	"testing"
)

func fakeInstall(t *testing.T, cacheDir, tag string) string {
	t.Helper()
	dir := filepath.Join(installRoot(cacheDir), tag)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, binaryName())
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o700); err != nil { // #nosec G306 -- stand-in binary
		t.Fatal(err)
	}
	return p
}

func TestNewestInstalled(t *testing.T) {
	cache := t.TempDir()

	if _, ok := newestInstalled(cache); ok {
		t.Fatal("empty cache should have no installs")
	}

	fakeInstall(t, cache, "1125-01-4588b13")
	want := fakeInstall(t, cache, "1205-01-e35c129")
	fakeInstall(t, cache, "1201-02-19c2728")
	// Noise that must be ignored: bad tag, missing binary.
	if err := os.MkdirAll(filepath.Join(installRoot(cache), "not-a-tag"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(installRoot(cache), "9999-99-fffffff"), 0o750); err != nil {
		t.Fatal(err)
	}

	got, ok := newestInstalled(cache)
	if !ok || got != want {
		t.Errorf("newestInstalled = (%q, %v), want %q", got, ok, want)
	}
}

func TestInstalledPath(t *testing.T) {
	cache := t.TempDir()
	b := build{Version: "1205-01", Commit: "e35c129"}

	if _, ok := installedPath(cache, b); ok {
		t.Error("not yet installed build reported present")
	}
	want := fakeInstall(t, cache, b.tag())
	got, ok := installedPath(cache, b)
	if !ok || got != want {
		t.Errorf("installedPath = (%q, %v), want %q", got, ok, want)
	}
}
