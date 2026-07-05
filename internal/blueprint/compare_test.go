package blueprint

import (
	"strings"
	"testing"
	"time"

	"github.com/zandoh/spanner/internal/gauge"
)

func TestBuildComparison(t *testing.T) {
	rep, err := gauge.ParseFile("../gauge/testdata/profilesets-1205.json")
	if err != nil {
		t.Fatal(err)
	}
	c := buildComparison(rep)
	if c == nil {
		t.Fatal("no comparison built from profileset fixture")
	}

	if len(c.Rows) != 3 {
		t.Fatalf("rows = %d, want baseline + 2 candidates", len(c.Rows))
	}
	if !c.Rows[0].IsBaseline {
		t.Error("first row must be the pinned baseline")
	}
	// Fixture: "Head +11 ilvl" gains (~+400 dps), "No head gem" loses.
	if c.Rows[1].Label != "Head +11 ilvl" || !c.Rows[1].Gain {
		t.Errorf("best candidate wrong: %+v", c.Rows[1])
	}
	if c.Rows[2].Label != "No head gem" || c.Rows[2].Gain {
		t.Errorf("worst candidate wrong: %+v", c.Rows[2])
	}
	if !strings.HasPrefix(c.Rows[1].Delta, "+") || !strings.HasPrefix(c.Rows[2].Delta, "-") {
		t.Errorf("delta signs wrong: %q / %q", c.Rows[1].Delta, c.Rows[2].Delta)
	}
	// The largest |delta| defines the scale.
	if c.Rows[2].WidthPct != 100 {
		t.Errorf("largest delta width = %v, want 100", c.Rows[2].WidthPct)
	}
}

func TestBuildComparisonNilWithoutProfilesets(t *testing.T) {
	rep, err := gauge.ParseFile("../gauge/testdata/midnight-1205.json")
	if err != nil {
		t.Fatal(err)
	}
	if buildComparison(rep) != nil {
		t.Error("comparison should be nil for a plain sim")
	}
}

func TestRenderComparisonSection(t *testing.T) {
	rep, err := gauge.ParseFile("../gauge/testdata/profilesets-1205.json")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := Render(&b, rep, Meta{GeneratedAt: time.Now()}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := b.String()
	// html/template escapes '+' to &#43; in text nodes.
	for _, want := range []string{"Comparison — DPS vs baseline", "Baseline (current)", "Head &#43;11 ilvl", "cmp-bar gain", "cmp-bar loss"} {
		if !strings.Contains(html, want) {
			t.Errorf("report missing %q", want)
		}
	}
}
