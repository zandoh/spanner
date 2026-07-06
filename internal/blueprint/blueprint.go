// Package blueprint renders a parsed sim into spanner's HTML report — a
// single self-contained file with no external assets, so it works from
// file:// and survives being tossed into a Discord channel.
package blueprint

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zandoh/spanner/internal/gauge"
)

//go:embed report.tmpl.html
var reportSrc string

var reportTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"add":   func(a, b float64) float64 { return a + b },
	"sub":   func(a, b float64) float64 { return a - b },
	"halve": func(a float64) float64 { return a / 2 },
}).Parse(reportSrc))

// Meta carries report provenance that isn't part of the sim data itself.
type Meta struct {
	GeneratedAt time.Time
	ProfileName string
}

type page struct {
	Title       string
	GeneratedAt string
	SimCVersion string
	GitRevision string
	BuildDate   string
	Iterations  int
	FightLength string
	ErrorPct    string
	Elapsed     string
	Comparison  *comparisonView
	Weights     *weightsView
	Player      playerView
}

type playerView struct {
	Name      string
	Spec      string
	Race      string
	Level     int
	DPS       string
	DPSErr    string
	DPSMin    string
	DPSMax    string
	DTPS      string
	HPS       string
	DPSChart  *chartView
	Resources []*chartView
	Abilities []abilityView
	Buffs     []buffView
	Gear      []gearView
	Talents   string
}

type abilityView struct {
	Label    string
	Pct      string
	DPS      string
	WidthPct float64
	Executes string
	PerExec  string
}

type buffView struct {
	Label     string
	Uptime    float64
	UptimeStr string
}

type gearView struct {
	Slot   string
	Name   string
	ILevel int
}

// Render writes the HTML report for the first player in rep.
func Render(w io.Writer, rep *gauge.Report, meta Meta) error {
	p := rep.Sim.Players[0]
	cd := p.CollectedData

	pg := page{
		Title:       p.Name + " — spanner report",
		GeneratedAt: meta.GeneratedAt.Format("2006-01-02 15:04 MST"),
		SimCVersion: rep.Version,
		GitRevision: rep.GitRevision,
		BuildDate:   rep.BuildDate,
		Iterations:  cd.DPS.Count,
		FightLength: fmtDuration(cd.FightLength.Mean),
		ErrorPct:    fmtErrorPct(cd.DPS),
		Elapsed:     fmt.Sprintf("%.1fs", rep.Sim.Statistics.ElapsedTimeSeconds),
		Comparison:  buildComparison(rep),
		Weights:     buildWeights(p.ScaleFactors),
		Player: playerView{
			Name:      p.Name,
			Spec:      p.Specialization,
			Race:      prettify(p.Race),
			Level:     p.Level,
			DPS:       fmtInt(cd.DPS.Mean),
			DPSErr:    fmtInt(cd.DPS.MeanStdDev),
			DPSMin:    fmtInt(cd.DPS.Min),
			DPSMax:    fmtInt(cd.DPS.Max),
			DTPS:      fmtCompact(cd.DTPS.Mean),
			HPS:       fmtInt(cd.HPS.Mean),
			DPSChart:  buildTimeline("Damage per second over the fight", "DPS", "teal", cd.TimelineDmg, 210),
			Resources: resourceCharts(cd.ResourceTimelines),
			Abilities: abilityViews(p.Stats),
			Buffs:     buffViews(p.Buffs),
			Gear:      gearViews(p.Gear),
			Talents:   p.Talents,
		},
	}
	if err := reportTmpl.Execute(w, pg); err != nil {
		return fmt.Errorf("blueprint: rendering report: %w", err)
	}
	return nil
}

// resourceCharts renders one small-multiple per combat resource, each with
// its own scale — runes (0–6) and runic power (0–100) on a shared axis would
// misstate both. Health timelines are the tank lens's material, not the DPS
// report's.
func resourceCharts(timelines map[string]gauge.Timeline) []*chartView {
	names := make([]string, 0, len(timelines))
	for name := range timelines {
		if name == "health" || name == "health_pct" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var charts []*chartView
	for _, name := range names {
		label := prettify(name)
		if c := buildTimeline(label, label, "brass", timelines[name], 130); c != nil {
			charts = append(charts, c)
		}
	}
	return charts
}

func abilityViews(stats []gauge.ActionStat) []abilityView {
	var dmg []gauge.ActionStat
	for _, s := range stats {
		if s.Type == "damage" && s.PortionAmount >= 0.005 {
			dmg = append(dmg, s)
		}
	}
	sort.Slice(dmg, func(i, j int) bool { return dmg[i].PortionAmount > dmg[j].PortionAmount })
	if len(dmg) > 12 {
		dmg = dmg[:12]
	}
	if len(dmg) == 0 {
		return nil
	}
	// Sub-actions share a spell_name (blood_boil vs blood_boil_boiling_point
	// are both "Blood Boil"); fall back to the internal name for repeats.
	seen := make(map[string]int, len(dmg))
	for _, s := range dmg {
		seen[s.SpellName]++
	}
	maxPortion := dmg[0].PortionAmount
	views := make([]abilityView, 0, len(dmg))
	for _, s := range dmg {
		label := s.SpellName
		if label == "" || seen[s.SpellName] > 1 {
			label = prettify(s.Name)
		}
		perExec := 0.0
		if s.NumExecutes.Mean > 0 {
			perExec = s.ActualAmount.Mean / s.NumExecutes.Mean
		}
		views = append(views, abilityView{
			Label:    label,
			Pct:      fmt.Sprintf("%.1f%%", s.PortionAmount*100),
			DPS:      fmtInt(s.PortionAPS.Mean),
			WidthPct: s.PortionAmount / maxPortion * 100,
			Executes: fmt.Sprintf("%.1f", s.NumExecutes.Mean),
			PerExec:  fmtCompact(perExec),
		})
	}
	return views
}

func buffViews(buffs []gauge.Buff) []buffView {
	var up []gauge.Buff
	for _, b := range buffs {
		if b.Uptime >= 2 {
			up = append(up, b)
		}
	}
	sort.Slice(up, func(i, j int) bool { return up[i].Uptime > up[j].Uptime })
	views := make([]buffView, 0, len(up))
	taken := make(map[string]bool, len(up))
	for _, b := range up {
		label := b.SpellName
		if label == "" {
			label = prettify(b.Name)
		}
		// SimC tracks some auras several times over (one entry per source);
		// keep the highest-uptime instance of each label.
		if taken[label] {
			continue
		}
		taken[label] = true
		if len(views) == 14 {
			break
		}
		views = append(views, buffView{
			Label:     label,
			Uptime:    math.Min(b.Uptime, 100),
			UptimeStr: fmt.Sprintf("%.1f%%", b.Uptime),
		})
	}
	return views
}

var slotOrder = []string{
	"head", "neck", "shoulders", "back", "chest", "wrists", "hands",
	"waist", "legs", "feet", "finger1", "finger2", "trinket1", "trinket2",
	"main_hand", "off_hand",
}

func gearViews(gear map[string]gauge.Item) []gearView {
	views := make([]gearView, 0, len(gear))
	seen := make(map[string]bool, len(gear))
	for _, slot := range slotOrder {
		if item, ok := gear[slot]; ok {
			views = append(views, gearView{Slot: prettify(slot), Name: prettify(item.Name), ILevel: item.ILevel})
			seen[slot] = true
		}
	}
	// Any slot the fixed order doesn't know about still shows up at the end.
	var rest []string
	for slot := range gear {
		if !seen[slot] {
			rest = append(rest, slot)
		}
	}
	sort.Strings(rest)
	for _, slot := range rest {
		item := gear[slot]
		views = append(views, gearView{Slot: prettify(slot), Name: prettify(item.Name), ILevel: item.ILevel})
	}
	return views
}

// prettify turns simc snake_case identifiers into title-cased labels.
func prettify(s string) string {
	words := strings.Split(s, "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// fmtInt renders a float as a thousands-separated integer: 69282.1 → "69,282".
func fmtInt(v float64) string {
	s := strconv.FormatInt(int64(math.Round(v)), 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		if len(s) > pre {
			b.WriteByte(',')
		}
	}
	for i := pre; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// fmtCompact renders large magnitudes in short form: 2130076 → "2.13M".
func fmtCompact(v float64) string {
	abs := math.Abs(v)
	switch {
	case abs >= 1e9:
		return fmt.Sprintf("%.2fB", v/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("%.2fM", v/1e6)
	case abs >= 1e4:
		return fmt.Sprintf("%.1fK", v/1e3)
	default:
		return fmtInt(v)
	}
}

func fmtDuration(seconds float64) string {
	total := int(math.Round(seconds))
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

func fmtErrorPct(dps gauge.Sample) string {
	if dps.Mean == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2f%%", dps.MeanStdDev/dps.Mean*100)
}
