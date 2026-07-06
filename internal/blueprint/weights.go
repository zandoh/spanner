package blueprint

import (
	"fmt"
	"math"
	"sort"
)

// weightsView ranks stat weights as magnitude bars. Weapon DPS is kept out
// of the bars — at ~10× any stat weight it would crush the scale — and shown
// as a footnote instead.
type weightsView struct {
	Rows []weightRow
	Wdps string // empty when the sim didn't report it
}

type weightRow struct {
	Stat     string
	Value    string
	WidthPct float64
	Negative bool
}

// statLabels maps SimC's scale-factor keys to display names.
var statLabels = map[string]string{
	"Str": "Strength", "Agi": "Agility", "Int": "Intellect", "Sta": "Stamina",
	"AP": "Attack Power", "SP": "Spell Power", "Crit": "Critical Strike",
	"Haste": "Haste", "Mastery": "Mastery", "Vers": "Versatility",
	"Armor": "Armor", "BonusArmor": "Bonus Armor", "Leech": "Leech",
	"Avoidance": "Avoidance", "Speed": "Speed",
}

func buildWeights(factors map[string]float64) *weightsView {
	if len(factors) == 0 {
		return nil
	}
	v := &weightsView{}
	maxAbs := 0.0
	type kv struct {
		stat string
		w    float64
	}
	var stats []kv
	for stat, w := range factors {
		if stat == "Wdps" {
			v.Wdps = fmt.Sprintf("%.2f", w)
			continue
		}
		stats = append(stats, kv{stat, w})
		maxAbs = math.Max(maxAbs, math.Abs(w))
	}
	if maxAbs == 0 {
		return nil
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].w > stats[j].w })
	for _, s := range stats {
		label := statLabels[s.stat]
		if label == "" {
			label = s.stat
		}
		v.Rows = append(v.Rows, weightRow{
			Stat:     label,
			Value:    fmt.Sprintf("%.2f", s.w),
			WidthPct: math.Abs(s.w) / maxAbs * 100,
			Negative: s.w < 0,
		})
	}
	return v
}
