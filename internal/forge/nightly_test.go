package forge

import (
	"os"
	"testing"
)

// The fixture is a trimmed copy of the real nightly index listing.
func TestParseIndexFixture(t *testing.T) {
	html, err := os.ReadFile("testdata/nightly-index.html")
	if err != nil {
		t.Fatal(err)
	}
	builds := parseIndex(string(html))

	if len(builds) != 12 {
		t.Fatalf("parsed %d builds, want 12 (4 versions × 3 platforms)", len(builds))
	}

	latestMac, ok := latest(builds, "macos")
	if !ok || latestMac.Filename != "simc-1205-01-macos-e35c129.dmg" {
		t.Errorf("latest macos = %+v, ok=%v", latestMac, ok)
	}
	if latestMac.Version != "1205-01" || latestMac.Commit != "e35c129" {
		t.Errorf("macos version/commit = %q/%q", latestMac.Version, latestMac.Commit)
	}
	if latestMac.tag() != "1205-01-e35c129" {
		t.Errorf("tag() = %q", latestMac.tag())
	}

	latestWin, ok := latest(builds, "win64")
	if !ok || latestWin.Filename != "simc-1205.01.e35c129-win64.7z" {
		t.Errorf("latest win64 = %+v, ok=%v", latestWin, ok)
	}

	if _, ok := latest(builds, "linux"); ok {
		t.Error("latest should find nothing for an OS with no builds")
	}
}

func TestLatestPrefersHigherVersion(t *testing.T) {
	builds := parseIndex(`
		<a href="simc-1125-01-macos-aaaaaaa.dmg">x</a>
		<a href="simc-1205-01-macos-bbbbbbb.dmg">x</a>
		<a href="simc-1205-02-macos-ccccccc.dmg">x</a>
	`)
	got, ok := latest(builds, "macos")
	if !ok || got.Commit != "ccccccc" {
		t.Errorf("got %+v, want the 1205-02 build", got)
	}
}
