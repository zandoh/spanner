// Package workshop is the character store: named /simc exports saved on
// disk so a character can be simmed again without re-pasting. One directory
// per character — the raw export is the source of truth, a small metadata
// file makes listing cheap.
package workshop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/zandoh/spanner/internal/profile"
)

const (
	profileFile = "profile.simc"
	metaFile    = "meta.json"
)

// Store is a directory of saved characters.
type Store struct {
	Dir string
}

// Entry is one saved character as shown in listings.
type Entry struct {
	Name    string    `json:"name"` // the user-chosen save name
	Class   string    `json:"class"`
	Spec    string    `json:"spec"`
	Race    string    `json:"race"`
	Level   int       `json:"level"`
	SavedAt time.Time `json:"saved_at"`
}

// DefaultDir is the platform config location for saved characters
// (e.g. ~/Library/Application Support/spanner/characters on macOS).
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "spanner", "characters"), nil
}

// Save stores p under name, overwriting any previous save of that name.
func (s *Store) Save(name string, p *profile.Profile) error {
	key, err := slugify(name)
	if err != nil {
		return err
	}
	dir := filepath.Join(s.Dir, key)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("workshop: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, profileFile), []byte(p.Raw), 0o600); err != nil {
		return fmt.Errorf("workshop: writing profile: %w", err)
	}
	meta := Entry{
		Name: name, Class: p.Class, Spec: p.Spec, Race: p.Race,
		Level: p.Level, SavedAt: time.Now().UTC(),
	}
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("workshop: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, metaFile), raw, 0o600); err != nil {
		return fmt.Errorf("workshop: writing metadata: %w", err)
	}
	return nil
}

// Load re-parses the saved export; the raw text is the source of truth.
func (s *Store) Load(name string) (*profile.Profile, error) {
	key, err := slugify(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(s.Dir, key, profileFile)) // #nosec G304 -- path is the store's own layout
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("workshop: no character saved as %q (see `spanner char list`)", name)
	}
	if err != nil {
		return nil, fmt.Errorf("workshop: %w", err)
	}
	defer func() { _ = f.Close() }()
	return profile.ParseExport(f)
}

// List returns saved characters, most recently saved first.
func (s *Store) List() ([]Entry, error) {
	dirs, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("workshop: %w", err)
	}
	var entries []Entry
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.Dir, d.Name(), metaFile)) // #nosec G304 -- the store's own layout
		if err != nil {
			continue // a half-written save shouldn't break listing
		}
		var e Entry
		if json.Unmarshal(raw, &e) != nil {
			continue
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].SavedAt.After(entries[j].SavedAt) })
	return entries, nil
}

// Remove deletes a saved character.
func (s *Store) Remove(name string) error {
	key, err := slugify(name)
	if err != nil {
		return err
	}
	dir := filepath.Join(s.Dir, key)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("workshop: no character saved as %q", name)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("workshop: %w", err)
	}
	return nil
}

// slugify maps a save name to its directory key: lowercase alphanumerics
// plus - and _, so names stay filesystem-safe and case-insensitive.
func slugify(name string) (string, error) {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("workshop: %q is not a usable character name", name)
	}
	return b.String(), nil
}
