package rig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const indexFile = "runs.jsonl"

// HistoryEntry is one finished run as recorded in the out dir's index.
type HistoryEntry struct {
	Time     time.Time `json:"time"`
	Display  string    `json:"display"`
	DPS      float64   `json:"dps"`
	DPSError float64   `json:"dps_error"`
	Weights  bool      `json:"weights,omitempty"`
	Compare  int       `json:"compare,omitempty"` // number of profileset candidates
	HTML     string    `json:"html"`
	JSON     string    `json:"json"`
}

// History reads the run index newest-first. A missing index is an empty
// history, not an error.
func History(outDir string) ([]HistoryEntry, error) {
	raw, err := os.ReadFile(filepath.Join(outDir, indexFile)) // #nosec G304 -- the caller's own out dir
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("rig: reading run index: %w", err)
	}
	var entries []HistoryEntry
	dec := json.NewDecoder(bytes.NewReader(raw))
	for dec.More() {
		var e HistoryEntry
		if err := dec.Decode(&e); err != nil {
			// A torn last line (crash mid-append) shouldn't hide history.
			break
		}
		entries = append(entries, e)
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, nil
}

// appendIndex records a finished run; failures are reported to the caller
// but should not fail the run itself.
func appendIndex(outDir string, e HistoryEntry) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(outDir, indexFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- the caller's own out dir
	if err != nil {
		return err
	}
	_, err = f.Write(append(raw, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
