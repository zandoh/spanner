package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveInputProfilePassthrough(t *testing.T) {
	in, err := resolveInput("profiles/MID1_Death_Knight_Blood.simc", "")
	if err != nil {
		t.Fatalf("resolveInput: %v", err)
	}
	defer in.cleanup()
	if in.path != "profiles/MID1_Death_Knight_Blood.simc" || in.stem != "MID1_Death_Knight_Blood" {
		t.Errorf("got path=%q stem=%q", in.path, in.stem)
	}
}

func TestResolveInputImport(t *testing.T) {
	export := "deathknight=\"Zandy\"\nspec=blood\nlevel=90\nhead=,id=249970\n"
	src := filepath.Join(t.TempDir(), "export.txt")
	if err := os.WriteFile(src, []byte(export), 0o600); err != nil {
		t.Fatal(err)
	}

	in, err := resolveInput("", src)
	if err != nil {
		t.Fatalf("resolveInput: %v", err)
	}
	if in.stem != "Zandy" || !strings.Contains(in.display, "blood") {
		t.Errorf("got stem=%q display=%q", in.stem, in.display)
	}

	// The temp file must hold the export verbatim, and cleanup must remove it.
	got, err := os.ReadFile(in.path)
	if err != nil {
		t.Fatalf("reading temp input: %v", err)
	}
	if string(got) != export {
		t.Error("temp input does not match export")
	}
	in.cleanup()
	if _, err := os.Stat(in.path); !os.IsNotExist(err) {
		t.Error("cleanup did not remove temp input")
	}
}

func TestResolveInputFlagValidation(t *testing.T) {
	if _, err := resolveInput("", ""); err == nil {
		t.Error("want error when neither flag is set")
	}
	if _, err := resolveInput("a.simc", "b.txt"); err == nil {
		t.Error("want error when both flags are set")
	}
}
