// Package gauge parses SimulationCraft json2 reports into a stable model.
//
// The json2 schema is produced by SimC's `json2=` output option and is not
// formally versioned beyond report_version; gauge is the single place the
// rest of spanner depends on its shape, so schema drift across SimC nightlies
// is absorbed here (guarded by golden-file tests in testdata).
package gauge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Report is the root of a SimC json2 document.
type Report struct {
	Version       string `json:"version"`
	ReportVersion string `json:"report_version"`
	BuildDate     string `json:"build_date"`
	GitRevision   string `json:"git_revision"`
	Sim           Sim    `json:"sim"`
}

// Sim holds the simulation options, actors, and run statistics.
type Sim struct {
	Options    Options    `json:"options"`
	Players    []Player   `json:"players"`
	Statistics Statistics `json:"statistics"`
}

// Options is the subset of sim options spanner reports on.
type Options struct {
	Iterations  int     `json:"iterations"`
	MaxTime     float64 `json:"max_time"`
	TargetError float64 `json:"target_error"`
	Threads     int     `json:"threads"`
}

// Statistics describes the run as a whole.
type Statistics struct {
	ElapsedTimeSeconds float64 `json:"elapsed_time_seconds"`
	TotalEvents        int64   `json:"total_events_processed"`
	SimulationLength   Sample  `json:"simulation_length"`
	RaidDPS            Sample  `json:"raid_dps"`
}

// Player is one simulated actor.
type Player struct {
	Name           string          `json:"name"`
	Race           string          `json:"race"`
	Level          int             `json:"level"`
	Specialization string          `json:"specialization"`
	Talents        string          `json:"talents"`
	CollectedData  CollectedData   `json:"collected_data"`
	Stats          []ActionStat    `json:"stats"`
	Gear           map[string]Item `json:"gear"`
	Buffs          []Buff          `json:"buffs"`
}

// CollectedData carries the per-iteration aggregates for an actor. DTPS and
// HPS are collected even in DPS sims, which is what makes a future tank
// survivability lens possible without forking SimC.
type CollectedData struct {
	FightLength Sample `json:"fight_length"`
	DPS         Sample `json:"dps"`
	DPSe        Sample `json:"dpse"`
	DTPS        Sample `json:"dtps"`
	HPS         Sample `json:"hps"`
}

// Sample is SimC's aggregate of one collected quantity across iterations.
// Not every quantity carries every field (e.g. dtps omits median/std_dev).
type Sample struct {
	Count      int     `json:"count"`
	Mean       float64 `json:"mean"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Median     float64 `json:"median"`
	StdDev     float64 `json:"std_dev"`
	MeanStdDev float64 `json:"mean_std_dev"`
}

// ActionStat is one ability's contribution.
type ActionStat struct {
	Name          string  `json:"name"`
	SpellName     string  `json:"spell_name"`
	Type          string  `json:"type"`
	School        string  `json:"school"`
	NumExecutes   Sample  `json:"num_executes"`
	PortionAmount float64 `json:"portion_amount"`
	PortionAPS    Sample  `json:"portion_aps"`
	ActualAmount  Sample  `json:"actual_amount"`
}

// Item is one equipped piece of gear.
type Item struct {
	Name    string `json:"name"`
	ILevel  int    `json:"ilevel"`
	Encoded string `json:"encoded_item"`
}

// Buff is one tracked aura with its uptime as a percentage of fight length.
type Buff struct {
	Name      string  `json:"name"`
	SpellName string  `json:"spell_name"`
	Uptime    float64 `json:"uptime"`
}

// Parse decodes a json2 report from r.
func Parse(r io.Reader) (*Report, error) {
	var rep Report
	if err := json.NewDecoder(r).Decode(&rep); err != nil {
		return nil, fmt.Errorf("gauge: decoding json2 report: %w", err)
	}
	if rep.ReportVersion == "" {
		return nil, errors.New("gauge: missing report_version; not a simc json2 report")
	}
	if len(rep.Sim.Players) == 0 {
		return nil, errors.New("gauge: report contains no players")
	}
	return &rep, nil
}

// ParseFile decodes the json2 report at path.
func ParseFile(path string) (*Report, error) {
	f, err := os.Open(path) // #nosec G304 -- path is spanner's own sim output file
	if err != nil {
		return nil, fmt.Errorf("gauge: %w", err)
	}
	defer f.Close()
	return Parse(f)
}
