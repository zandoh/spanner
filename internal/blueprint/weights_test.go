package blueprint

import (
	"strings"
	"testing"
	"time"

	"github.com/zandoh/spanner/internal/gauge"
)

func TestBuildWeights(t *testing.T) {
	v := buildWeights(map[string]float64{
		"Str": 22.4, "Crit": 12.3, "Haste": 9.7, "Armor": -0.15, "Wdps": 124.6,
	})
	if v == nil {
		t.Fatal("nil weights view")
	}
	if v.Wdps != "124.60" {
		t.Errorf("Wdps = %q", v.Wdps)
	}
	if len(v.Rows) != 4 {
		t.Fatalf("rows = %d, want 4 (Wdps excluded)", len(v.Rows))
	}
	if v.Rows[0].Stat != "Strength" || v.Rows[0].WidthPct != 100 {
		t.Errorf("top row = %+v", v.Rows[0])
	}
	last := v.Rows[len(v.Rows)-1]
	if last.Stat != "Armor" || !last.Negative {
		t.Errorf("last row = %+v", last)
	}

	if buildWeights(nil) != nil {
		t.Error("nil factors should give nil view")
	}
}

func TestRenderWeightsSection(t *testing.T) {
	rep, err := gauge.ParseFile("../gauge/testdata/weights-1205.json")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := Render(&b, rep, Meta{GeneratedAt: time.Now()}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"Stat weights — DPS per point", "Critical Strike", "Weapon DPS"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("report missing %q", want)
		}
	}
}
