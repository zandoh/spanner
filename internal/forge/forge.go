// Package forge manages the SimulationCraft binary: locating one to run and
// fetching nightly builds into a managed cache.
package forge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// EnvVar names the environment variable that can point at a simc binary.
const EnvVar = "SPANNER_SIMC"

// Locate resolves a runnable simc binary, in order of preference: the
// explicit path (e.g. a --simc flag), the SPANNER_SIMC environment variable,
// the newest build in the managed cache (which spanner keeps current, unlike
// a hand-installed PATH copy), then PATH. cacheDir may be empty to skip the
// cache.
func Locate(explicit, cacheDir string) (string, error) {
	if explicit != "" {
		return checkBinary(explicit)
	}
	if env := os.Getenv(EnvVar); env != "" {
		return checkBinary(env)
	}
	if cacheDir != "" {
		if path, ok := newestInstalled(cacheDir); ok {
			return path, nil
		}
	}
	if path, err := exec.LookPath("simc"); err == nil {
		return path, nil
	}
	return "", errors.New("forge: no simc binary found: run `spanner forge update` to install the latest nightly, pass -simc, set " + EnvVar + ", or put simc on PATH")
}

func checkBinary(path string) (string, error) {
	info, err := os.Stat(path) // #nosec G703 -- the user pointing spanner at their own simc binary is the feature
	if err != nil {
		return "", fmt.Errorf("forge: simc binary: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("forge: simc binary %s is a directory", path)
	}
	return path, nil
}
