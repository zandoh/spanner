package profile

import (
	"strings"
	"testing"
)

// addonExport mirrors the shape of a real /simc addon paste: identity block,
// commented sections, gear lines, and a commented "gear from bags" alternate
// whose values must not win over the active character's.
const addonExport = `deathknight="Zandy"
level=90
race=mechagnome
region=us
server=illidan
role=tank
professions=engineering=100/blacksmithing=25
spec=blood
talents=CoPAAAAAAAAAAAAAAAAAAAAAAwYWmZmxMmZmhZZmZmmZxMjxMAAA

# saved on 2026-07-04
head=,id=249970,gem_id=240983
neck=,id=252010
main_hand=,id=237723

### Gear from Bags
# head=,id=111111
# spec=frost
`

func TestParseExport(t *testing.T) {
	p, err := ParseExport(strings.NewReader(addonExport))
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}

	want := Profile{
		Class:   "deathknight",
		Name:    "Zandy",
		Spec:    "blood",
		Race:    "mechagnome",
		Level:   90,
		Region:  "us",
		Server:  "illidan",
		Talents: "CoPAAAAAAAAAAAAAAAAAAAAAAwYWmZmxMmZmhZZmZmmZxMjxMAAA",
	}
	got := *p
	got.Raw = ""
	if got != want {
		t.Errorf("parsed identity\n got: %+v\nwant: %+v", got, want)
	}
	if p.Raw != addonExport {
		t.Error("Raw must preserve the export verbatim")
	}
}

func TestParseExportUnquotedName(t *testing.T) {
	p, err := ParseExport(strings.NewReader("warrior=Bob\nlevel=80\n"))
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}
	if p.Class != "warrior" || p.Name != "Bob" || p.Level != 80 {
		t.Errorf("got %+v", p)
	}
}

func TestParseExportRejectsNonProfiles(t *testing.T) {
	for name, doc := range map[string]string{
		"empty":         "",
		"comments only": "# just a comment\n",
		"random text":   "hello world\nfoo=bar\n",
	} {
		if _, err := ParseExport(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestSlug(t *testing.T) {
	cases := []struct{ name, want string }{
		{"Zandy", "Zandy"},
		{"MID1_Death_Knight_Blood_San'layn", "MID1_Death_Knight_Blood_Sanlayn"},
		{"Åströ", "str"},
		{"'''", "character"},
	}
	for _, c := range cases {
		p := Profile{Name: c.name}
		if got := p.Slug(); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
