// Package forge manages the SimulationCraft binary. Phase 0 only locates an
// existing binary; fetching and verifying nightlies comes later.
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
// then PATH.
func Locate(explicit string) (string, error) {
	if explicit != "" {
		return checkBinary(explicit)
	}
	if env := os.Getenv(EnvVar); env != "" {
		return checkBinary(env)
	}
	if path, err := exec.LookPath("simc"); err == nil {
		return path, nil
	}
	return "", errors.New("forge: no simc binary found: pass --simc, set " + EnvVar + ", or put simc on PATH")
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
