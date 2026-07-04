// Package crank runs SimulationCraft: it turns a profile into a simc
// invocation, supervises the process, and leaves a json2 report on disk.
package crank

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Options tune a sim run. Zero values defer to SimC's own defaults.
type Options struct {
	Iterations  int
	Threads     int
	TargetError float64
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

	cmd := exec.CommandContext(ctx, simcPath, args...) // #nosec G204 -- running the user's own simc binary on their profile is the product
	cmd.Stdout = progress
	cmd.Stderr = progress
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("crank: simc run failed: %w", err)
	}
	return nil
}
