package blueprint

import (
	"fmt"
	"math"
	"sort"

	"github.com/zandoh/spanner/internal/gauge"
)

// comparisonView ranks profileset candidates against the baseline actor as
// diverging bars: gains grow right from a zero line, losses grow left —
// zero-based magnitude bars would render a 1% upgrade invisible.
type comparisonView struct {
	Metric string
	Rows   []compareRow
}

type compareRow struct {
	Label      string
	Mean       string
	Delta      string
	Detail     string // tooltip: ± error and iterations
	IsBaseline bool
	Gain       bool
	WidthPct   float64 // |delta| as a share of the largest |delta|, 0–100

	delta float64 // numeric delta for ranking
}

// buildComparison returns nil when the report has no profilesets.
func buildComparison(rep *gauge.Report) *comparisonView {
	results := rep.Sim.Profilesets.Results
	if len(results) == 0 {
		return nil
	}
	base := rep.Sim.Players[0].CollectedData.DPS

	maxAbs := 0.0
	for _, r := range results {
		maxAbs = math.Max(maxAbs, math.Abs(r.Mean-base.Mean))
	}
	if maxAbs == 0 {
		maxAbs = 1
	}

	rows := make([]compareRow, 0, len(results)+1)
	rows = append(rows, compareRow{
		Label:      "Baseline (current)",
		Mean:       fmtInt(base.Mean),
		Delta:      "—",
		Detail:     fmt.Sprintf("± %s · %d iterations", fmtInt(base.MeanStdDev), base.Count),
		IsBaseline: true,
	})
	for _, r := range results {
		delta := r.Mean - base.Mean
		rows = append(rows, compareRow{
			Label:    r.Name,
			Mean:     fmtInt(r.Mean),
			Delta:    fmt.Sprintf("%s · %s%%", fmtSigned(delta), fmtSignedPct(delta/base.Mean*100)),
			Detail:   fmt.Sprintf("± %s · %d iterations", fmtInt(r.MeanError), r.Iterations),
			Gain:     delta >= 0,
			WidthPct: math.Abs(delta) / maxAbs * 100,
			delta:    delta,
		})
	}
	// Baseline pinned first, candidates ranked best-first below it.
	sort.SliceStable(rows[1:], func(i, j int) bool {
		return rows[1:][i].delta > rows[1:][j].delta
	})
	return &comparisonView{Metric: "DPS", Rows: rows}
}

func fmtSigned(v float64) string {
	if v >= 0 {
		return "+" + fmtInt(v)
	}
	return fmtInt(v)
}

func fmtSignedPct(v float64) string {
	return fmt.Sprintf("%+.2f", v)
}
