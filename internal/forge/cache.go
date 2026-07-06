package forge

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
)

// binaryName is the simc executable's filename inside an install dir.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "simc.exe"
	}
	return "simc"
}

// DefaultCacheDir is where forge keeps managed simc installs
// (e.g. ~/Library/Caches/spanner on macOS).
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "spanner"), nil
}

// installRoot is the directory holding one subdirectory per installed build,
// named by build.tag ("1205-01-e35c129").
func installRoot(cacheDir string) string {
	return filepath.Join(cacheDir, "simc")
}

func installDir(cacheDir string, b build) string {
	return filepath.Join(installRoot(cacheDir), b.tag())
}

// installedPath returns the binary path for a build if it is installed.
func installedPath(cacheDir string, b build) (string, bool) {
	p := filepath.Join(installDir(cacheDir, b), binaryName())
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		return p, true
	}
	return "", false
}

var tagRe = regexp.MustCompile(`^(\d+)-(\d+)-[0-9a-f]+$`)

// newestInstalled scans the cache for the highest-versioned usable binary.
func newestInstalled(cacheDir string) (string, bool) {
	entries, err := os.ReadDir(installRoot(cacheDir))
	if err != nil {
		return "", false
	}
	var (
		bestPath         string
		bestMaj, bestMin int
		found            bool
	)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m := tagRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		p := filepath.Join(installRoot(cacheDir), e.Name(), binaryName())
		if info, err := os.Stat(p); err != nil || info.IsDir() {
			continue
		}
		maj, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		if !found || maj > bestMaj || (maj == bestMaj && min > bestMin) {
			bestPath, bestMaj, bestMin, found = p, maj, min, true
		}
	}
	return bestPath, found
}
