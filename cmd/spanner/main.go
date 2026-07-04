// Command spanner is the terminal client: run a sim, get a report.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/zandoh/spanner/internal/blueprint"
	"github.com/zandoh/spanner/internal/crank"
	"github.com/zandoh/spanner/internal/forge"
	"github.com/zandoh/spanner/internal/gauge"
)

const simTimeout = 30 * time.Minute

func main() {
	if len(os.Args) < 2 || os.Args[1] != "sim" {
		fmt.Fprintln(os.Stderr, "usage: spanner sim -profile <file.simc> [-simc <path>] [-iterations N] [-threads N] [-target-error PCT] [-out DIR] [-open=false]")
		os.Exit(2)
	}
	if err := runSim(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "spanner:", err)
		os.Exit(1)
	}
}

func runSim(args []string) error {
	fs := flag.NewFlagSet("sim", flag.ExitOnError)
	simcFlag := fs.String("simc", "", "path to the simc binary (default: $"+forge.EnvVar+", then PATH)")
	profile := fs.String("profile", "", "path to a .simc profile (required)")
	iterations := fs.Int("iterations", 0, "sim iterations (default: simc's own default)")
	threads := fs.Int("threads", 0, "sim threads (default: all cores)")
	targetError := fs.Float64("target-error", 0, "stop at this DPS error percentage")
	outDir := fs.String("out", "reports", "directory for the report and raw json")
	openReport := fs.Bool("open", true, "open the report in a browser when done")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *profile == "" {
		return fmt.Errorf("-profile is required")
	}

	simcPath, err := forge.Locate(*simcFlag)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*outDir, 0o750); err != nil {
		return err
	}
	stem := strings.TrimSuffix(filepath.Base(*profile), filepath.Ext(*profile))
	stem = fmt.Sprintf("%s-%s", stem, time.Now().Format("20060102-150405"))
	jsonPath := filepath.Join(*outDir, stem+".simc.json")
	htmlPath := filepath.Join(*outDir, stem+".html")

	ctx, cancel := context.WithTimeout(context.Background(), simTimeout)
	defer cancel()

	fmt.Fprintf(os.Stderr, "⚙ spanner: cranking %s with %s\n", filepath.Base(*profile), simcPath)
	opts := crank.Options{Iterations: *iterations, Threads: *threads, TargetError: *targetError}
	if err := crank.Run(ctx, simcPath, *profile, jsonPath, opts, os.Stderr); err != nil {
		return err
	}

	rep, err := gauge.ParseFile(jsonPath)
	if err != nil {
		return err
	}

	out, err := os.Create(htmlPath) // #nosec G304 -- report path derives from the user's -out flag
	if err != nil {
		return err
	}
	meta := blueprint.Meta{GeneratedAt: time.Now(), ProfileName: filepath.Base(*profile)}
	if err := blueprint.Render(out, rep, meta); err != nil {
		_ = out.Close() // render error is the one worth reporting
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	fmt.Printf("report: %s\nraw json: %s\n", htmlPath, jsonPath)
	if *openReport {
		if err := openInBrowser(htmlPath); err != nil {
			fmt.Fprintln(os.Stderr, "spanner: could not open browser:", err)
		}
	}
	return nil
}

func openInBrowser(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	// #nosec G204 -- opening spanner's own report file with the platform opener
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", abs) // #nosec G204
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", abs) // #nosec G204
	default:
		cmd = exec.Command("xdg-open", abs) // #nosec G204
	}
	return cmd.Start()
}
