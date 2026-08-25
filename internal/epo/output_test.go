package epo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dphcko/internal/tax"
)

func TestWritePeriodReplacesFilesAndRemovesStaleKH(t *testing.T) {
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
	zero, err := WritePeriod(dir, testProfile(), 2026, 8, tax.Summary{}, now)
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
