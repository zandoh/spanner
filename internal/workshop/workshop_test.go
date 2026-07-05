package workshop

import (
	"strings"
	"testing"

	"github.com/zandoh/spanner/internal/profile"
)

const export = `deathknight="Zandy"
spec=blood
race=mechagnome
level=90
head=,id=249970
`

func testProfile(t *testing.T) *profile.Profile {
	t.Helper()
	p, err := profile.ParseExport(strings.NewReader(export))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSaveLoadRoundTrip(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.Save("Zandy Main", testProfile(t)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load is case-insensitive via slugging.
	got, err := s.Load("zandy main")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Raw != export {
		t.Error("loaded profile does not round-trip the export verbatim")
	}
	if got.Class != "deathknight" || got.Spec != "blood" {
		t.Errorf("re-parsed identity wrong: %+v", got)
	}
}

func TestSaveOverwrites(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.Save("main", testProfile(t)); err != nil {
		t.Fatal(err)
	}
	updated := testProfile(t)
	updated.Raw = strings.Replace(export, "level=90", "level=91", 1)
	if err := s.Save("main", updated); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("main")
	if err != nil {
		t.Fatal(err)
	}
	if got.Level != 91 {
		t.Errorf("overwrite not effective: level = %d", got.Level)
	}
}

func TestListAndRemove(t *testing.T) {
	s := &Store{Dir: t.TempDir()}

	if entries, err := s.List(); err != nil || entries != nil {
		t.Fatalf("empty store: got (%v, %v)", entries, err)
	}

	for _, name := range []string{"main", "alt"} {
		if err := s.Save(name, testProfile(t)); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := s.List()
	if err != nil || len(entries) != 2 {
		t.Fatalf("List: got %d entries, err %v", len(entries), err)
	}
	if entries[0].Class != "deathknight" || entries[0].Level != 90 {
		t.Errorf("listing metadata wrong: %+v", entries[0])
	}

	if err := s.Remove("main"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := s.Load("main"); err == nil {
		t.Error("Load succeeded after Remove")
	}
	if err := s.Remove("main"); err == nil {
		t.Error("removing a missing character should error")
	}
}

func TestSlugify(t *testing.T) {
	if _, err := slugify("!!!"); err == nil {
		t.Error("unusable name should error")
	}
	got, err := slugify("Zandy's Main-Tank")
	if err != nil || got != "zandysmain-tank" {
		t.Errorf("slugify = (%q, %v)", got, err)
	}
}
