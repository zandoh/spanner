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
	"github.com/zandoh/spanner/internal/profile"
)

const simTimeout = 30 * time.Minute

func main() {
	if len(os.Args) < 2 || os.Args[1] != "sim" {
		fmt.Fprintln(os.Stderr, "usage: spanner sim (-profile <file.simc> | -import <export.txt|->) [-simc <path>] [-iterations N] [-threads N] [-target-error PCT] [-out DIR] [-open=false]")
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
	profileFlag := fs.String("profile", "", "path to a .simc profile")
	importFlag := fs.String("import", "", "path to a /simc addon export, or - for stdin")
	iterations := fs.Int("iterations", 0, "sim iterations (default: simc's own default)")
	threads := fs.Int("threads", 0, "sim threads (default: all cores)")
	targetError := fs.Float64("target-error", 0, "stop at this DPS error percentage")
	outDir := fs.String("out", "reports", "directory for the report and raw json")
	openReport := fs.Bool("open", true, "open the report in a browser when done")
	if err := fs.Parse(args); err != nil {
		return err
	}
	input, err := resolveInput(*profileFlag, *importFlag)
	if err != nil {
		return err
	}
	defer input.cleanup()

	simcPath, err := forge.Locate(*simcFlag)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*outDir, 0o750); err != nil {
		return err
	}
	stem := fmt.Sprintf("%s-%s", input.stem, time.Now().Format("20060102-150405"))
	jsonPath := filepath.Join(*outDir, stem+".simc.json")
	htmlPath := filepath.Join(*outDir, stem+".html")

	ctx, cancel := context.WithTimeout(context.Background(), simTimeout)
	defer cancel()

	fmt.Fprintf(os.Stderr, "⚙ spanner: cranking %s with %s\n", input.display, simcPath)
	opts := crank.Options{Iterations: *iterations, Threads: *threads, TargetError: *targetError}
	if err := crank.Run(ctx, simcPath, input.path, jsonPath, opts, os.Stderr); err != nil {
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
	meta := blueprint.Meta{GeneratedAt: time.Now(), ProfileName: input.display}
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

// simInput is a resolved sim source: a path SimC can execute, plus naming
// for output files and progress messages.
type simInput struct {
	path    string
	stem    string
	display string
	cleanup func()
}

// resolveInput turns the -profile / -import flags into a runnable input.
// Imports are validated and written to a temp file for SimC to consume.
func resolveInput(profilePath, importPath string) (*simInput, error) {
	switch {
	case profilePath != "" && importPath != "":
		return nil, fmt.Errorf("-profile and -import are mutually exclusive")
	case profilePath != "":
		stem := strings.TrimSuffix(filepath.Base(profilePath), filepath.Ext(profilePath))
		return &simInput{path: profilePath, stem: stem, display: filepath.Base(profilePath), cleanup: func() {}}, nil
	case importPath != "":
		return resolveImport(importPath)
	default:
		return nil, fmt.Errorf("one of -profile or -import is required")
	}
}

func resolveImport(importPath string) (*simInput, error) {
	src := os.Stdin
	if importPath != "-" {
		f, err := os.Open(importPath) // #nosec G304 -- the user's own export file
		if err != nil {
			return nil, err
		}
		defer f.Close()
		src = f
	}
	prof, err := profile.ParseExport(src)
	if err != nil {
		return nil, err
	}

	tmp, err := os.CreateTemp("", "spanner-*.simc")
	if err != nil {
		return nil, err
	}
	if _, err := tmp.WriteString(prof.Raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return nil, err
	}

	display := prof.Name
	if prof.Spec != "" {
		display = fmt.Sprintf("%s (%s %s)", prof.Name, prof.Spec, prof.Class)
	}
	return &simInput{
		path:    tmp.Name(),
		stem:    prof.Slug(),
		display: display,
		cleanup: func() { _ = os.Remove(tmp.Name()) },
	}, nil
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
