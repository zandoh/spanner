// Package crank runs SimulationCraft: it turns a profile into a simc
// invocation, supervises the process, and leaves a json2 report on disk.
package crank

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// waitDelay bounds how long Run waits for simc's output pipes to close after
// the process is killed. Without it, anything the dead process leaves holding
// the pipe (an orphaned child, a stuck thread) blocks Run for the orphan's
// lifetime instead of the kill being prompt.
const waitDelay = 2 * time.Second

// Options tune a sim run. Zero values defer to SimC's own defaults.
type Options struct {
	Iterations  int
	Threads     int
	TargetError float64
	// ScaleFactors makes SimC compute stat weights — one extra sim per
	// stat, so runs take several times longer.
	ScaleFactors bool
}

// Run executes simc on the profile at profilePath, writing the json2 report
// to jsonPath. SimC's console output (including its progress ticker) streams
// to progress. The process is killed if ctx is cancelled.
func Run(ctx context.Context, simcPath, profilePath, jsonPath string, opts Options, progress io.Writer) error {
	args := []string{profilePath, "json2=" + jsonPath}
	if opts.Iterations > 0 {
		args = append(args, fmt.Sprintf("iterations=%d", opts.Iterations))
	}
	if opts.Threads > 0 {
		args = append(args, fmt.Sprintf("threads=%d", opts.Threads))
	}
	if opts.TargetError > 0 {
		args = append(args, fmt.Sprintf("target_error=%g", opts.TargetError))
	}
	if opts.ScaleFactors {
		args = append(args, "calculate_scale_factors=1")
	}

	cmd := exec.CommandContext(ctx, simcPath, args...) // #nosec G204 -- running the user's own simc binary on their profile is the product
	cmd.Stdout = progress
	cmd.Stderr = progress
	cmd.WaitDelay = waitDelay
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("crank: simc run failed: %w", err)
	}
	return nil
}
