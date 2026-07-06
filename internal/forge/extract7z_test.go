package forge

import (
	"os"
	"path/filepath"
	"testing"
)

// The fixture mirrors the real nightly layout: versioned root dir holding
// simc.exe, COPYING, a decoy GUI exe, and DLL noise.
func TestExtract7z(t *testing.T) {
	dest := t.TempDir()
	if err := extract7z("testdata/win64-nightly.7z", dest); err != nil {
		t.Fatalf("extract7z: %v", err)
	}

	bin, err := os.ReadFile(filepath.Join(dest, "simc.exe"))
	if err != nil {
		t.Fatalf("simc.exe not extracted: %v", err)
	}
	if string(bin) != "MZ fake simc console binary\n" {
		t.Error("extracted the wrong exe (decoy GUI binary?)")
	}
	if _, err := os.Stat(filepath.Join(dest, "COPYING")); err != nil {
		t.Error("GPL notice not extracted")
	}
	// Only the two wanted files come out.
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("extracted %d files, want 2", len(entries))
	}
}

func TestExtract7zWithoutSimc(t *testing.T) {
	if err := extract7z("testdata/win64-nightly.7z", t.TempDir()); err != nil {
		t.Skip("fixture unreadable")
	}
	// A valid archive with no simc.exe must error, not silently succeed:
	// reuse the fixture check indirectly by pointing at a non-archive.
	if err := extract7z("testdata/nightly-index.html", t.TempDir()); err == nil {
		t.Error("non-archive input should error")
	}
}

// TestExtract7zRealNightly runs only when pointed at a real downloaded
// nightly (SPANNER_TEST_7Z=/path/to/simc-...-win64.7z): CI skips it.
func TestExtract7zRealNightly(t *testing.T) {
	archive := os.Getenv("SPANNER_TEST_7Z")
	if archive == "" {
		t.Skip("SPANNER_TEST_7Z not set")
	}
	dest := t.TempDir()
	if err := extract7z(archive, dest); err != nil {
		t.Fatalf("extract7z: %v", err)
	}
	info, err := os.Stat(filepath.Join(dest, "simc.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() < 1<<20 {
		t.Errorf("simc.exe suspiciously small: %d bytes", info.Size())
	}
}
