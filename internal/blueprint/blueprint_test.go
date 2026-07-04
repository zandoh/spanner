package blueprint

import (
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/zandoh/spanner/internal/gauge"
)

func TestRenderFixture(t *testing.T) {
	rep, err := gauge.ParseFile("../gauge/testdata/midnight-1205.json")
	if err != nil {
		t.Fatalf("parsing fixture: %v", err)
	}

	var b strings.Builder
	meta := Meta{GeneratedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC), ProfileName: "fixture.simc"}
	if err := Render(&b, rep, meta); err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := b.String()

	for _, want := range []string{
		// The fixture name contains an apostrophe, which html/template escapes.
		template.HTMLEscapeString(rep.Sim.Players[0].Name),
		"Damage by ability",
		"Buff uptime",
		"Equipped gear",
		"Damage per second",
		rep.Version,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report missing %q", want)
		}
	}
	if strings.Contains(html, "<no value>") {
		t.Error("report contains unresolved template fields")
	}
}

func TestFormatting(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{fmtInt(69282.19), "69,282"},
		{fmtInt(999), "999"},
		{fmtInt(1000), "1,000"},
		{fmtInt(0), "0"},
		{fmtCompact(2130076), "2.13M"},
		{fmtCompact(25293041952), "25.29B"},
		{fmtCompact(43703), "43.7K"},
		{fmtCompact(999), "999"},
		{fmtDuration(300.09), "5:00"},
		{fmtDuration(359.76), "6:00"},
		{prettify("heart_strike"), "Heart Strike"},
		{prettify("main_hand"), "Main Hand"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
