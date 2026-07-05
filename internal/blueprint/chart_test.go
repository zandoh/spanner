package blueprint

import (
	"strings"
	"testing"

	"github.com/zandoh/spanner/internal/gauge"
)

func TestBuildTimeline(t *testing.T) {
	tl := gauge.Timeline{Data: []float64{0, 25000, 50000, 75000, 60000, 40000}}
	c := buildTimeline("DPS over time", "DPS", "teal", tl, 210)
	if c == nil {
		t.Fatal("buildTimeline returned nil for real data")
	}

	if !strings.HasPrefix(c.Line, "M") || strings.Count(c.Line, "L") != len(tl.Data)-1 {
		t.Errorf("line path malformed: %q", c.Line)
	}
	if !strings.HasSuffix(c.Area, "Z") {
		t.Errorf("area path not closed: %q", c.Area)
	}
	// y ticks must start at 0 and cover the max (75000 → 20000 steps to 80000).
	if c.YTicks[0].Label != "0" {
		t.Errorf("first y tick = %q, want 0", c.YTicks[0].Label)
	}
	if last := c.YTicks[len(c.YTicks)-1].Label; last != "80K" {
		t.Errorf("last y tick = %q, want 80K", last)
	}
	if !strings.HasPrefix(c.SeriesJSON, "[[") {
		t.Errorf("series json malformed: %.40q", c.SeriesJSON)
	}
}

func TestBuildTimelineRejectsEmptyAndFlat(t *testing.T) {
	if c := buildTimeline("t", "u", "teal", gauge.Timeline{}, 210); c != nil {
		t.Error("empty timeline should render no chart")
	}
	if c := buildTimeline("t", "u", "teal", gauge.Timeline{Data: []float64{0, 0, 0}}, 210); c != nil {
		t.Error("flat-zero timeline should render no chart")
	}
}

func TestNiceStep(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{1.5, 2}, {18750, 20000}, {3, 5}, {0.9, 1}, {60, 100}, {24, 25},
	}
	for _, c := range cases {
		if got := niceStep(c.in); got != c.want {
			t.Errorf("niceStep(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestResourceChartsSkipHealth(t *testing.T) {
	charts := resourceCharts(map[string]gauge.Timeline{
		"health":      {Data: []float64{1, 2}},
		"health_pct":  {Data: []float64{100, 100}},
		"runic_power": {Data: []float64{0, 50, 80}},
		"rune":        {Data: []float64{6, 4, 5}},
	})
	if len(charts) != 2 {
		t.Fatalf("got %d charts, want 2 (health excluded)", len(charts))
	}
	if charts[0].Title != "Rune" || charts[1].Title != "Runic Power" {
		t.Errorf("titles = %q, %q", charts[0].Title, charts[1].Title)
	}
}
