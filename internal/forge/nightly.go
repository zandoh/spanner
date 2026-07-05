package forge

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
)

// defaultBaseURL is the upstream nightly distribution directory. It is plain
// HTTP: the server's TLS certificate does not match the hostname, upstream
// publishes no checksums, and there is no other official binary channel —
// forge records a SHA-256 of everything it downloads as the best available
// integrity measure.
const defaultBaseURL = "http://downloads.simulationcraft.org/nightly/"

// build is one downloadable nightly artifact.
type build struct {
	Filename string
	Version  string // e.g. "1205-01": game patch 12.0.5, build 01
	Commit   string // short upstream git sha
	OS       string // "macos", "win64", or "winarm64"

	major, minor int
}

// tag is the build's cache-directory name, unique per nightly.
func (b build) tag() string {
	return fmt.Sprintf("%s-%s", b.Version, b.Commit)
}

// The two filename shapes in the nightly index:
//
//	simc-1205-01-macos-e35c129.dmg
//	simc-1205.01.e35c129-win64.7z
var (
	macBuildRe = regexp.MustCompile(`simc-(\d+)-(\d+)-macos-([0-9a-f]+)\.dmg`)
	winBuildRe = regexp.MustCompile(`simc-(\d+)\.(\d+)\.([0-9a-f]+)-(win64|winarm64)\.7z`)
)

// parseIndex extracts the nightly builds advertised in the distribution
// directory's HTML listing. Duplicate filenames (the listing links each file
// more than once) are collapsed.
func parseIndex(html string) []build {
	var builds []build
	seen := make(map[string]bool)

	add := func(filename string, m []string, os string) {
		if seen[filename] {
			return
		}
		seen[filename] = true
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		builds = append(builds, build{
			Filename: filename,
			Version:  fmt.Sprintf("%s-%s", m[1], m[2]),
			Commit:   m[3],
			OS:       os,
			major:    major,
			minor:    minor,
		})
	}

	for _, m := range macBuildRe.FindAllStringSubmatch(html, -1) {
		add(m[0], m, "macos")
	}
	for _, m := range winBuildRe.FindAllStringSubmatch(html, -1) {
		add(m[0], m, m[4])
	}
	return builds
}

// latest returns the newest build for the given OS token, by version number.
func latest(builds []build, os string) (build, bool) {
	var best build
	found := false
	for _, b := range builds {
		if b.OS != os {
			continue
		}
		if !found || b.major > best.major || (b.major == best.major && b.minor > best.minor) {
			best, found = b, true
		}
	}
	return best, found
}

// platformOS maps the running platform to the index's OS token. Empty means
// upstream ships no binaries for this platform (notably Linux, which builds
// from source).
func platformOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		if runtime.GOARCH == "arm64" {
			return "winarm64"
		}
		return "win64"
	default:
		return ""
	}
}
