package forge

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Fetcher downloads and installs simc nightlies into the cache.
type Fetcher struct {
	BaseURL  string       // nightly index URL; defaultBaseURL if empty
	CacheDir string       // spanner cache root; required
	Client   *http.Client // defaults to a client with a generous timeout
	Progress io.Writer    // human-readable status lines; may be nil
}

// manifest records what was installed and the hash forge observed at
// download time — upstream publishes no checksums, so this is trust-on-
// first-use, recorded for later auditing rather than verification.
type manifest struct {
	Version   string    `json:"version"`
	Commit    string    `json:"commit"`
	Filename  string    `json:"filename"`
	SourceURL string    `json:"source_url"`
	SHA256    string    `json:"sha256"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Update installs the newest nightly for this platform and returns the
// binary path. Already-installed builds are reused without downloading.
func (f *Fetcher) Update(ctx context.Context) (string, error) {
	osToken := platformOS()
	if osToken == "" {
		return "", errors.New("forge: upstream ships no binaries for this platform; build simc from source (https://github.com/simulationcraft/simc) and use -simc or PATH")
	}
	if f.CacheDir == "" {
		return "", errors.New("forge: fetcher needs a cache dir")
	}

	base := f.BaseURL
	if base == "" {
		base = defaultBaseURL
	}

	index, err := f.getString(ctx, base)
	if err != nil {
		return "", fmt.Errorf("forge: fetching nightly index: %w", err)
	}
	bld, ok := latest(parseIndex(index), osToken)
	if !ok {
		return "", fmt.Errorf("forge: nightly index lists no %s builds", osToken)
	}

	if path, ok := installedPath(f.CacheDir, bld); ok {
		f.logf("simc %s is already the newest nightly", bld.tag())
		return path, nil
	}

	fileURL, err := url.JoinPath(base, bld.Filename)
	if err != nil {
		return "", fmt.Errorf("forge: %w", err)
	}
	f.logf("downloading %s ...", fileURL)
	artifact, sum, err := f.download(ctx, fileURL)
	if err != nil {
		return "", fmt.Errorf("forge: downloading %s: %w", bld.Filename, err)
	}
	defer func() { _ = os.Remove(artifact) }()

	dir := installDir(f.CacheDir, bld)
	if err := extract(bld, artifact, dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("forge: extracting %s: %w", bld.Filename, err)
	}

	m := manifest{
		Version:   bld.Version,
		Commit:    bld.Commit,
		Filename:  bld.Filename,
		SourceURL: fileURL,
		SHA256:    sum,
		FetchedAt: time.Now().UTC(),
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("forge: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600); err != nil {
		return "", fmt.Errorf("forge: writing manifest: %w", err)
	}

	path := filepath.Join(dir, binaryName)
	f.logf("installed simc %s (sha256 %.12s…)", bld.tag(), sum)
	return path, nil
}

func (f *Fetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return &http.Client{Timeout: 15 * time.Minute}
}

func (f *Fetcher) logf(format string, args ...any) {
	if f.Progress != nil {
		fmt.Fprintf(f.Progress, "⚙ forge: "+format+"\n", args...)
	}
}

func (f *Fetcher) get(ctx context.Context, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return resp, nil
}

func (f *Fetcher) getString(ctx context.Context, u string) (string, error) {
	resp, err := f.get(ctx, u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// download streams u to a temp file, hashing as it goes.
func (f *Fetcher) download(ctx context.Context, u string) (path, sha string, err error) {
	resp, err := f.get(ctx, u)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	tmp, err := os.CreateTemp("", "spanner-forge-*")
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	_, err = io.Copy(tmp, io.TeeReader(resp.Body, h))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", "", err
	}
	return tmp.Name(), fmt.Sprintf("%x", h.Sum(nil)), nil
}

func extract(b build, artifact, destDir string) error {
	switch b.OS {
	case "macos":
		return extractDMG(artifact, destDir)
	default:
		return fmt.Errorf("no extractor for %s builds yet; use -simc or PATH", b.OS)
	}
}

// extractDMG mounts the nightly disk image and copies out the simc CLI
// binary plus the GPL notice that must travel with it.
func extractDMG(dmgPath, destDir string) error {
	mnt, err := os.MkdirTemp("", "spanner-dmg-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(mnt) }()

	attach := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-quiet", "-mountpoint", mnt, dmgPath) // #nosec G204 -- fixed args, paths owned by forge
	if out, err := attach.CombinedOutput(); err != nil {
		return fmt.Errorf("hdiutil attach: %v: %s", err, out)
	}
	defer func() {
		detach := exec.Command("hdiutil", "detach", "-quiet", mnt) // #nosec G204 -- fixed args
		if err := detach.Run(); err != nil {
			// A busy mount detaches on a later retry; -force as fallback.
			_ = exec.Command("hdiutil", "detach", "-quiet", "-force", mnt).Run() // #nosec G204
		}
	}()

	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(mnt, binaryName), filepath.Join(destDir, binaryName), 0o750); err != nil {
		return fmt.Errorf("copying simc binary: %w", err)
	}
	// GPL-3.0 requires the license to accompany the binary; best effort.
	_ = copyFile(filepath.Join(mnt, "COPYING"), filepath.Join(destDir, "COPYING"), 0o640)
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src) // #nosec G304 -- paths owned by forge
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm) // #nosec G304 -- paths owned by forge
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
