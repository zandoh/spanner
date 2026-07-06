// Package rig assembles the whole machine for one sim run: locate a simc
// binary, execute it, parse the json2 output, and render the report. The
// terminal client and the local web server both drive this same pipeline.
package rig

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/zandoh/spanner/internal/blueprint"
	"github.com/zandoh/spanner/internal/crank"
	"github.com/zandoh/spanner/internal/forge"
	"github.com/zandoh/spanner/internal/gauge"
)

// Job describes one sim run.
type Job struct {
	SimcPath  string // explicit binary override; empty uses env/cache/PATH
	InputPath string // the .simc input SimC executes
	Stem      string // output filename stem; timestamped by Run
	Display   string // human-readable name for progress and the report
	Options   crank.Options
	OutDir    string    // where the report and raw json land
	Progress  io.Writer // simc console output; nil discards it
}

// Result is a finished run.
type Result struct {
	Report   *gauge.Report
	HTMLPath string
	JSONPath string
}

// Run executes the pipeline. The returned paths exist on success.
func Run(ctx context.Context, job Job) (*Result, error) {
	progress := job.Progress
	if progress == nil {
		progress = io.Discard
	}

	cacheDir, _ := forge.DefaultCacheDir() // empty on error: Locate skips the cache
	simcPath, err := forge.Locate(job.SimcPath, cacheDir)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(job.OutDir, 0o750); err != nil {
		return nil, err
	}
	stem := fmt.Sprintf("%s-%s", job.Stem, time.Now().Format("20060102-150405"))
	jsonPath := filepath.Join(job.OutDir, stem+".simc.json")
	htmlPath := filepath.Join(job.OutDir, stem+".html")

	_, _ = fmt.Fprintf(progress, "⚙ spanner: cranking %s with %s\n", job.Display, simcPath)
	if err := crank.Run(ctx, simcPath, job.InputPath, jsonPath, job.Options, progress); err != nil {
		return nil, err
	}

	rep, err := gauge.ParseFile(jsonPath)
	if err != nil {
		return nil, err
	}

	out, err := os.Create(htmlPath) // #nosec G304 -- path derives from the caller's out dir
	if err != nil {
		return nil, err
	}
	meta := blueprint.Meta{GeneratedAt: time.Now(), ProfileName: job.Display}
	if err := blueprint.Render(out, rep, meta); err != nil {
		_ = out.Close() // render error is the one worth reporting
		return nil, err
	}
	if err := out.Close(); err != nil {
		return nil, err
	}

	cd := rep.Sim.Players[0].CollectedData
	entry := HistoryEntry{
		Time:     time.Now().UTC(),
		Display:  job.Display,
		DPS:      cd.DPS.Mean,
		DPSError: cd.DPS.MeanStdDev,
		Weights:  job.Options.ScaleFactors,
		Compare:  len(rep.Sim.Profilesets.Results),
		HTML:     filepath.Base(htmlPath),
		JSON:     filepath.Base(jsonPath),
	}
	if err := appendIndex(job.OutDir, entry); err != nil {
		// History is a convenience; the run itself succeeded.
		fmt.Fprintf(progress, "⚙ spanner: could not record run history: %v\n", err)
	}

	return &Result{Report: rep, HTMLPath: htmlPath, JSONPath: jsonPath}, nil
}
