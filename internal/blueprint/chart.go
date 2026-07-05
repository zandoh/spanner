package blueprint

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/zandoh/spanner/internal/gauge"
)

// chartView is a rendered SVG line chart: paths and ticks are precomputed in
// pixel space so the template stays dumb and the JS crosshair only needs the
// embedded series data.
type chartView struct {
	Title  string
	Unit   string
	Class  string // series color class: "teal" or "brass"
	W, H   int
	PlotX  float64
	PlotW  float64
	PlotY  float64
	PlotH  float64
	Line   string
	Area   string
	YTicks []tickView
	XTicks []tickView
	// SeriesJSON carries [display value, y-pixel] pairs for the crosshair.
	SeriesJSON string
}

type tickView struct {
	Pos   float64
	Label string
}

const (
	chartW     = 800
	marginL    = 60
	marginR    = 14
	marginTop  = 12
	marginBot  = 26
	xTickEvery = 60 // seconds
)

// buildTimeline renders one per-second series. Returns nil when the series
// is empty or flat-zero (nothing worth plotting).
func buildTimeline(title, unit, class string, tl gauge.Timeline, height int) *chartView {
	data := tl.Data
	if len(data) < 2 {
		return nil
	}
	maxV := 0.0
	for _, v := range data {
		maxV = math.Max(maxV, v)
	}
	if maxV <= 0 {
		return nil
	}

	c := &chartView{
		Title: title, Unit: unit, Class: class,
		W: chartW, H: height,
		PlotX: marginL, PlotW: float64(chartW - marginL - marginR),
		PlotY: marginTop, PlotH: float64(height - marginTop - marginBot),
	}

	step := niceStep(maxV / 4)
	yMax := math.Ceil(maxV/step) * step
	for v := 0.0; v <= yMax+step/2; v += step {
		c.YTicks = append(c.YTicks, tickView{Pos: c.yPix(v, yMax), Label: fmtAxis(v)})
	}
	for s := 0; s < len(data); s += xTickEvery {
		c.XTicks = append(c.XTicks, tickView{Pos: c.xPix(s, len(data)), Label: fmtDuration(float64(s))})
	}

	var line, area strings.Builder
	ys := make([][2]float64, len(data))
	for i, v := range data {
		x, y := c.xPix(i, len(data)), c.yPix(v, yMax)
		if i == 0 {
			fmt.Fprintf(&line, "M%.1f %.1f", x, y)
		} else {
			fmt.Fprintf(&line, " L%.1f %.1f", x, y)
		}
		ys[i] = [2]float64{math.Round(v*10) / 10, math.Round(y*10) / 10}
	}
	baseline := c.PlotY + c.PlotH
	fmt.Fprintf(&area, "%s L%.1f %.1f L%.1f %.1f Z", line.String(), c.xPix(len(data)-1, len(data)), baseline, c.PlotX, baseline)
	c.Line, c.Area = line.String(), area.String()

	raw, err := json.Marshal(ys)
	if err != nil {
		return nil // marshaling a float slice cannot realistically fail
	}
	c.SeriesJSON = string(raw)
	return c
}

func (c *chartView) xPix(i, n int) float64 {
	return c.PlotX + float64(i)/float64(n-1)*c.PlotW
}

func (c *chartView) yPix(v, yMax float64) float64 {
	return c.PlotY + c.PlotH - v/yMax*c.PlotH
}

// niceStep rounds raw up to a clean tick interval (1/2/2.5/5 × 10^k).
func niceStep(raw float64) float64 {
	if raw <= 0 {
		return 1
	}
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if raw <= m*mag {
			return m * mag
		}
	}
	return 10 * mag
}

// fmtAxis keeps y-tick labels clean: compact for big values (no trailing
// zero decimals — "600K", not "600.0K"), no fake precision for small ones.
func fmtAxis(v float64) string {
	switch {
	case v >= 10000:
		s := fmtCompact(v)
		s = strings.Replace(s, ".00", "", 1)
		return strings.Replace(s, ".0", "", 1)
	case v == math.Trunc(v):
		return fmtInt(v)
	default:
		return fmt.Sprintf("%.1f", v)
	}
}
