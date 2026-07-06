package rig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryRoundTrip(t *testing.T) {
	dir := t.TempDir()

	if entries, err := History(dir); err != nil || entries != nil {
		t.Fatalf("empty dir: got (%v, %v)", entries, err)
	}

	for i, dps := range []float64{68000, 69500, 70123} {
		e := HistoryEntry{
			Time: time.Date(2026, 7, 5, 12, i, 0, 0, time.UTC), Display: "Zandy",
			DPS: dps, DPSError: 70, HTML: "r.html", JSON: "r.json",
		}
		if err := appendIndex(dir, e); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := History(dir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("History: %d entries, err %v", len(entries), err)
	}
	if entries[0].DPS != 70123 {
		t.Errorf("newest first: got %v", entries[0].DPS)
	}
}

func TestHistoryToleratesTornLine(t *testing.T) {
	dir := t.TempDir()
	if err := appendIndex(dir, HistoryEntry{Display: "ok", DPS: 1}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, indexFile), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"display":"torn`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	entries, err := History(dir)
	if err != nil || len(entries) != 1 || entries[0].Display != "ok" {
		t.Errorf("torn line handling: %v, %v", entries, err)
	}
}

func TestRunRecordsHistory(t *testing.T) {
	outDir := t.TempDir()
	_, err := Run(t.Context(), Job{
		SimcPath:  fakeSimc(t),
		InputPath: "x.simc",
		Stem:      "zandy",
		Display:   "Zandy",
		OutDir:    outDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := History(outDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("history after run: %d entries, err %v", len(entries), err)
	}
	if entries[0].DPS <= 0 || entries[0].HTML == "" {
		t.Errorf("entry incomplete: %+v", entries[0])
	}
}
