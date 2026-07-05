package profile

import (
	"strings"
	"testing"
)

func TestParseVariation(t *testing.T) {
	v, err := ParseVariation("Crafted boots=feet=,id=219911;waist=,id=219910")
	if err != nil {
		t.Fatalf("ParseVariation: %v", err)
	}
	if v.Label != "Crafted boots" || len(v.Overrides) != 2 {
		t.Errorf("got %+v", v)
	}
	if v.Overrides[0] != "feet=,id=219911" || v.Overrides[1] != "waist=,id=219910" {
		t.Errorf("overrides = %v", v.Overrides)
	}

	for _, bad := range []string{"", "nolabel", "Label=", "=talents=X", "Label=;;"} {
		if _, err := ParseVariation(bad); err == nil {
			t.Errorf("ParseVariation(%q) should fail", bad)
		}
	}
}

func TestWithProfilesets(t *testing.T) {
	p := &Profile{Raw: "deathknight=\"Zandy\"\nspec=blood"}
	got := p.WithProfilesets([]Variation{
		{Label: "A", Overrides: []string{"talents=XYZ"}},
		{Label: `B "quoted"`, Overrides: []string{"feet=,id=1", "waist=,id=2"}},
	})

	wantLines := []string{
		"deathknight=\"Zandy\"",
		"spec=blood",
		`profileset."A"=talents=XYZ`,
		`profileset."B 'quoted'"=feet=,id=1`,
		`profileset."B 'quoted'"+=waist=,id=2`,
	}
	for _, w := range wantLines {
		if !strings.Contains(got, w+"\n") {
			t.Errorf("missing line %q in:\n%s", w, got)
		}
	}
	// Baseline must come first and unmodified.
	if !strings.HasPrefix(got, p.Raw+"\n") {
		t.Error("baseline text was altered")
	}
}
