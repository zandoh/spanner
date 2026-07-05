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
	"text/tabwriter"
	"time"

	"github.com/zandoh/spanner/internal/blueprint"
	"github.com/zandoh/spanner/internal/crank"
	"github.com/zandoh/spanner/internal/forge"
	"github.com/zandoh/spanner/internal/gauge"
	"github.com/zandoh/spanner/internal/profile"
	"github.com/zandoh/spanner/internal/workshop"
)

const simTimeout = 30 * time.Minute

const usage = `usage:
  spanner sim (-profile <file.simc> | -import <export.txt|-> | -char <name>)
              [-simc <path>] [-iterations N] [-threads N] [-target-error PCT]
              [-out DIR] [-open=false]
  spanner char save <name> [-import <export.txt|->]   save a character (default: stdin)
  spanner char list                                   list saved characters
  spanner char rm <name>                              delete a saved character
  spanner forge update    download the latest simc nightly into the cache
  spanner forge which     print the simc binary a sim would use`

func main() {
	var err error
	switch {
	case len(os.Args) >= 2 && os.Args[1] == "sim":
		err = runSim(os.Args[2:])
	case len(os.Args) >= 2 && os.Args[1] == "char":
		err = runChar(os.Args[2:])
	case len(os.Args) >= 2 && os.Args[1] == "forge":
		err = runForge(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "spanner:", err)
		os.Exit(1)
	}
}

func defaultStore() (*workshop.Store, error) {
	dir, err := workshop.DefaultDir()
	if err != nil {
		return nil, fmt.Errorf("resolving character store: %w", err)
	}
	return &workshop.Store{Dir: dir}, nil
}

func runChar(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: spanner char (save <name> [-import <file|->] | list | rm <name>)")
	}
	store, err := defaultStore()
	if err != nil {
		return err
	}
	switch args[0] {
	case "save":
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return fmt.Errorf("usage: spanner char save <name> [-import <file|->]")
		}
		name := args[1]
		fs := flag.NewFlagSet("char save", flag.ExitOnError)
		importFlag := fs.String("import", "-", "path to a /simc addon export, or - for stdin")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		prof, err := readExport(*importFlag)
		if err != nil {
			return err
		}
		if err := store.Save(name, prof); err != nil {
			return err
		}
		fmt.Printf("saved %s — %s (%s %s, level %d)\n", name, prof.Name, prof.Spec, prof.Class, prof.Level)
		return nil
	case "list":
		entries, err := store.List()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("no saved characters; try: pbpaste | spanner char save <name>")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tSPEC\tCLASS\tLEVEL\tSAVED")
		for _, e := range entries {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", e.Name, e.Spec, e.Class, e.Level, e.SavedAt.Local().Format("2006-01-02 15:04"))
		}
		return w.Flush()
	case "rm":
		if len(args) != 2 {
			return fmt.Errorf("usage: spanner char rm <name>")
		}
		if err := store.Remove(args[1]); err != nil {
			return err
		}
		fmt.Println("removed", args[1])
		return nil
	default:
		return fmt.Errorf("unknown char command %q; want save, list, or rm", args[0])
	}
}

func runForge(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: spanner forge (update|which)")
	}
	cacheDir, err := forge.DefaultCacheDir()
	if err != nil {
		return fmt.Errorf("resolving cache dir: %w", err)
	}
	switch args[0] {
	case "update":
		ctx, cancel := context.WithTimeout(context.Background(), simTimeout)
		defer cancel()
		fetcher := forge.Fetcher{CacheDir: cacheDir, Progress: os.Stderr}
		path, err := fetcher.Update(ctx)
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	case "which":
		path, err := forge.Locate("", cacheDir)
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	default:
		return fmt.Errorf("unknown forge command %q; want update or which", args[0])
	}
}

func runSim(args []string) error {
	fs := flag.NewFlagSet("sim", flag.ExitOnError)
	simcFlag := fs.String("simc", "", "path to the simc binary (default: $"+forge.EnvVar+", then PATH)")
	profileFlag := fs.String("profile", "", "path to a .simc profile")
	importFlag := fs.String("import", "", "path to a /simc addon export, or - for stdin")
	charFlag := fs.String("char", "", "name of a saved character (see spanner char list)")
	iterations := fs.Int("iterations", 0, "sim iterations (default: simc's own default)")
	threads := fs.Int("threads", 0, "sim threads (default: all cores)")
	targetError := fs.Float64("target-error", 0, "stop at this DPS error percentage")
	outDir := fs.String("out", "reports", "directory for the report and raw json")
	openReport := fs.Bool("open", true, "open the report in a browser when done")
	if err := fs.Parse(args); err != nil {
		return err
	}
	input, err := resolveInput(*profileFlag, *importFlag, *charFlag)
	if err != nil {
		return err
	}
	defer input.cleanup()

	cacheDir, _ := forge.DefaultCacheDir() // empty on error: Locate just skips the cache
	simcPath, err := forge.Locate(*simcFlag, cacheDir)
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

// resolveInput turns the -profile / -import / -char flags into a runnable
// input. Imports and saved characters are written to a temp file for SimC.
func resolveInput(profilePath, importPath, charName string) (*simInput, error) {
	set := 0
	for _, v := range []string{profilePath, importPath, charName} {
		if v != "" {
			set++
		}
	}
	switch {
	case set > 1:
		return nil, fmt.Errorf("-profile, -import, and -char are mutually exclusive")
	case profilePath != "":
		stem := strings.TrimSuffix(filepath.Base(profilePath), filepath.Ext(profilePath))
		return &simInput{path: profilePath, stem: stem, display: filepath.Base(profilePath), cleanup: func() {}}, nil
	case importPath != "":
		prof, err := readExport(importPath)
		if err != nil {
			return nil, err
		}
		return profileInput(prof)
	case charName != "":
		store, err := defaultStore()
		if err != nil {
			return nil, err
		}
		prof, err := store.Load(charName)
		if err != nil {
			return nil, err
		}
		return profileInput(prof)
	default:
		return nil, fmt.Errorf("one of -profile, -import, or -char is required")
	}
}

// readExport parses a /simc export from a file path or stdin ("-").
func readExport(path string) (*profile.Profile, error) {
	src := os.Stdin
	if path != "-" {
		f, err := os.Open(path) // #nosec G304 -- the user's own export file
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		src = f
	}
	return profile.ParseExport(src)
}

// profileInput materializes a parsed profile as a temp file SimC can run.
func profileInput(prof *profile.Profile) (*simInput, error) {
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
