package forge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
)

// extract7z pulls the simc CLI binary and the GPL notice out of a Windows
// nightly. The archive is the full Qt GUI bundle (DLLs, GUI exe); only
// simc.exe and COPYING are wanted — the console binary runs standalone.
func extract7z(archivePath, destDir string) error {
	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("opening 7z: %w", err)
	}
	defer func() { _ = r.Close() }()

	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return err
	}
	var found bool
	for _, f := range r.File {
		base := filepath.Base(strings.ReplaceAll(f.Name, `\`, "/"))
		var dest string
		var perm os.FileMode
		switch base {
		case "simc.exe":
			dest, perm = filepath.Join(destDir, "simc.exe"), os.FileMode(0o750)
			found = true
		case "COPYING":
			dest, perm = filepath.Join(destDir, "COPYING"), os.FileMode(0o640)
		default:
			continue
		}
		if err := extract7zFile(f, dest, perm); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("no simc.exe in %s", filepath.Base(archivePath))
	}
	return nil
}

func extract7zFile(f *sevenzip.File, dest string, perm os.FileMode) error {
	in, err := f.Open()
	if err != nil {
		return fmt.Errorf("extracting %s: %w", f.Name, err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm) // #nosec G304 -- dest is forge's own install dir
	if err != nil {
		return err
	}
	// The nightly holds one bounded binary; no decompression-bomb exposure
	// worth a limit here.
	_, err = io.Copy(out, in) // #nosec G110
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
