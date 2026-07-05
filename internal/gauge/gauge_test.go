package gauge

import (
	"strings"
	"testing"
)

// TestParseFixture guards the json2 fields spanner depends on against schema
// drift: the fixture is real output from a SimC nightly, and this test is the
// tripwire when a new nightly changes shape.
func TestParseFixture(t *testing.T) {
	rep, err := ParseFile("testdata/midnight-1205.json")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if rep.ReportVersion != "2.0.0" {
		t.Errorf("report_version = %q, want 2.0.0", rep.ReportVersion)
	}
	if rep.Version == "" || rep.GitRevision == "" {
		t.Errorf("missing build identity: version=%q git=%q", rep.Version, rep.GitRevision)
	}

	if len(rep.Sim.Players) != 1 {
		t.Fatalf("players = %d, want 1", len(rep.Sim.Players))
	}
	p := rep.Sim.Players[0]
	if !strings.Contains(p.Specialization, "Blood") {
		t.Errorf("specialization = %q, want a Blood Death Knight", p.Specialization)
	}
	if p.Level == 0 || p.Race == "" || p.Talents == "" {
		t.Errorf("player identity incomplete: level=%d race=%q talents empty=%v", p.Level, p.Race, p.Talents == "")
	}

	cd := p.CollectedData
	if cd.DPS.Mean <= 0 || cd.DPS.Count == 0 {
		t.Errorf("dps sample not populated: %+v", cd.DPS)
	}
	if cd.DPS.MeanStdDev <= 0 {
		t.Errorf("dps mean_std_dev not populated: %+v", cd.DPS)
	}
	// DTPS/HPS existing in a plain DPS sim is what the future tank lens
	// relies on — fail loudly if a nightly stops collecting them.
	if cd.DTPS.Mean <= 0 {
		t.Errorf("dtps sample not populated: %+v", cd.DTPS)
	}
	if cd.HPS.Mean <= 0 {
		t.Errorf("hps sample not populated: %+v", cd.HPS)
	}
	if cd.FightLength.Mean < 60 {
		t.Errorf("fight_length mean = %v, implausibly short", cd.FightLength.Mean)
	}

	// Timelines should be per-second series roughly matching fight length.
	if n := len(cd.TimelineDmg.Data); float64(n) < cd.FightLength.Mean-5 {
		t.Errorf("timeline_dmg has %d points for a %.0fs fight", n, cd.FightLength.Mean)
	}
	if n := len(cd.TimelineDmgTaken.Data); float64(n) < cd.FightLength.Mean-5 {
		t.Errorf("timeline_dmg_taken has %d points for a %.0fs fight", n, cd.FightLength.Mean)
	}
	if len(cd.ResourceTimelines) == 0 {
		t.Error("no resource timelines parsed")
	}
	for _, key := range []string{"health", "runic_power"} {
		if tl, ok := cd.ResourceTimelines[key]; !ok || len(tl.Data) == 0 {
			t.Errorf("resource timeline %q missing or empty", key)
		}
	}

	var damage int
	for _, s := range p.Stats {
		if s.Type == "damage" && s.PortionAmount > 0 {
			damage++
			if s.Name == "" {
				t.Errorf("damage stat with empty name: %+v", s)
			}
		}
	}
	if damage < 5 {
		t.Errorf("damage-contributing stats = %d, want at least 5", damage)
	}

	for _, slot := range []string{"head", "chest", "legs", "main_hand"} {
		if _, ok := p.Gear[slot]; !ok {
			t.Errorf("gear missing slot %q", slot)
		}
	}
	if len(p.Buffs) == 0 {
		t.Error("no buffs parsed")
	}

	if rep.Sim.Statistics.ElapsedTimeSeconds <= 0 {
		t.Errorf("statistics.elapsed_time_seconds = %v", rep.Sim.Statistics.ElapsedTimeSeconds)
	}
}

func TestParseRejectsNonReports(t *testing.T) {
	for name, doc := range map[string]string{
		"empty object": `{}`,
		"no players":   `{"report_version":"2.0.0","sim":{"players":[]}}`,
		"not json":     `simc says hi`,
	} {
		if _, err := Parse(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: Parse accepted invalid input", name)
		}
	}
}
