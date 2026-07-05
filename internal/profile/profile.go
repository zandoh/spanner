// Package profile handles character input. A /simc addon export string is
// already native SimulationCraft TCI input — SimC's own parser consumes it
// directly — so spanner never re-implements gear or talent parsing. This
// package only extracts identity fields (for display, file naming, and the
// future profile store) and carries the text through untouched.
package profile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// classTokens are the actor-creation keys SimC accepts; the first line using
// one of them names the character and marks the text as a simc profile.
var classTokens = map[string]bool{
	"deathknight": true, "demonhunter": true, "druid": true, "evoker": true,
	"hunter": true, "mage": true, "monk": true, "paladin": true,
	"priest": true, "rogue": true, "shaman": true, "warlock": true,
	"warrior": true,
}

// Profile is a character ready to sim: identity fields for presentation plus
// the raw TCI text that SimC executes.
type Profile struct {
	Class   string // simc class token, e.g. "deathknight"
	Name    string
	Spec    string
	Race    string
	Level   int
	Region  string
	Server  string
	Talents string
	Raw     string // the untouched input; valid simc TCI
}

// ParseExport reads a /simc addon export (or any simc character profile) and
// extracts its identity. The raw text is preserved verbatim in Raw.
func ParseExport(r io.Reader) (*Profile, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("profile: reading export: %w", err)
	}

	p := &Profile{Raw: string(raw)}
	sc := bufio.NewScanner(strings.NewReader(p.Raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = unquote(strings.TrimSpace(value))

		if classTokens[key] && p.Class == "" {
			p.Class = key
			p.Name = value
			continue
		}
		// First occurrence wins: the active character always precedes any
		// alternate blocks in an addon export.
		switch key {
		case "spec":
			if p.Spec == "" {
				p.Spec = value
			}
		case "race":
			if p.Race == "" {
				p.Race = value
			}
		case "level":
			if p.Level == 0 {
				if n, err := strconv.Atoi(value); err == nil {
					p.Level = n
				}
			}
		case "region":
			if p.Region == "" {
				p.Region = value
			}
		case "server":
			if p.Server == "" {
				p.Server = value
			}
		case "talents":
			if p.Talents == "" {
				p.Talents = value
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("profile: scanning export: %w", err)
	}
	if p.Class == "" {
		return nil, errors.New("profile: no character line found; expected a /simc addon export (e.g. deathknight=\"Name\")")
	}
	return p, nil
}

// Slug is a filesystem-safe form of the character name for output files.
func (p *Profile) Slug() string {
	var b strings.Builder
	for _, r := range p.Name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "character"
	}
	return b.String()
}

func unquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return s[1 : len(s)-1]
	}
	return s
}
