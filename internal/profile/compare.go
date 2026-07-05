package profile

import (
	"fmt"
	"strings"
)

// Variation is one comparison candidate: a label and the TCI overrides that
// distinguish it from the baseline (e.g. a talents= string or gear lines).
type Variation struct {
	Label     string
	Overrides []string
}

// ParseVariation reads a -vs flag value of the form
//
//	Label=override[;override...]
//
// e.g. "Deathbringer=talents=CoPA..." or "Crafted boots=feet=,id=219911;waist=,id=219910".
// The label is everything before the first '=' and may not contain one.
func ParseVariation(s string) (Variation, error) {
	label, rest, ok := strings.Cut(s, "=")
	label = strings.TrimSpace(label)
	if !ok || label == "" {
		return Variation{}, fmt.Errorf("profile: variation %q must look like Label=override[;override...]", s)
	}
	var overrides []string
	for _, o := range strings.Split(rest, ";") {
		if o = strings.TrimSpace(o); o != "" {
			overrides = append(overrides, o)
		}
	}
	if len(overrides) == 0 {
		return Variation{}, fmt.Errorf("profile: variation %q has no overrides", s)
	}
	return Variation{Label: label, Overrides: overrides}, nil
}

// WithProfilesets appends profileset definitions to a baseline profile,
// producing input for a SimC comparison run. The baseline text is untouched;
// SimC itself applies each variation's overrides on top of it.
func (p *Profile) WithProfilesets(vars []Variation) string {
	var b strings.Builder
	b.WriteString(p.Raw)
	if !strings.HasSuffix(p.Raw, "\n") {
		b.WriteByte('\n')
	}
	for _, v := range vars {
		label := strings.ReplaceAll(v.Label, `"`, `'`)
		for i, o := range v.Overrides {
			op := "="
			if i > 0 {
				op = "+="
			}
			fmt.Fprintf(&b, "profileset.\"%s\"%s%s\n", label, op, o)
		}
	}
	return b.String()
}
