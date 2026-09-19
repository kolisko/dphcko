package epo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dphcko/internal/tax"
)

func TestWritePeriodRefusesToOverwriteExistingFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	paths, err := WritePeriod(dir, testProfile(), 2026, 8, testSummary(t), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.DPH, paths.KH, paths.Summary} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("výstup %s: %v", path, err)
		}
	}
	_, err = WritePeriod(dir, testProfile(), 2026, 8, tax.Summary{}, now)
	if !errors.Is(err, ErrOutputExists) {
		t.Fatalf("opakovaný zápis má skončit ErrOutputExists, dostal jsem %v", err)
	}
}

func TestReplacePeriodReplacesFilesAndRemovesStaleKH(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	if _, err := WritePeriod(dir, testProfile(), 2026, 8, testSummary(t), now); err != nil {
		t.Fatal(err)
	}
	existing, err := ExistingPeriodOutputs(dir, 2026, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(existing) != 3 {
		t.Fatalf("nalezeno %d existujících výstupů, chci 3: %v", len(existing), existing)
	}

	zero, err := ReplacePeriod(dir, testProfile(), 2026, 8, tax.Summary{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if zero.KH != "" {
		t.Fatalf("nulové období vrátilo cestu KH %s", zero.KH)
	}
	staleKH := filepath.Join(dir, "vystup", "DPHKH1_2026-08.xml")
	if _, err := os.Stat(staleKH); !os.IsNotExist(err) {
		t.Fatalf("staré KH zůstalo na disku: %v", err)
	}
}
