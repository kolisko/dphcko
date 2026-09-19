package idoklad

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := State{LastDocumentNumber: "20260042", IssuerVATID: "CZ9001010007"}
	if err := saveState(root, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("stav = %#v, chci %#v", got, want)
	}
	info, err := os.Stat(filepath.Join(root, ".dphcko", stateFileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("oprávnění stavu = %o, chci 600", info.Mode().Perm())
	}
}

func TestNaturalDocumentNumberOrdering(t *testing.T) {
	values := []string{"20260010", "20260002", "20260001"}
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if !naturalLess(values[j], values[i]) {
				t.Fatalf("%q má být před %q", values[j], values[i])
			}
		}
	}
	if !naturalLess("FV-2025-999", "FV-2026-001") {
		t.Fatal("přechod na novou číselnou řadu roku musí být novější")
	}
}

func TestMissingDocumentNumbersKeepsRemoteOrder(t *testing.T) {
	got := missingDocumentNumbers([]string{"20260004", "20260003", "20260002"}, []string{"20260003"})
	want := []string{"20260004", "20260002"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chybějící = %#v, chci %#v", got, want)
	}
}

func TestLatestLocalDocumentIgnoresPDFOutsidePeriodDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "examples"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "examples", "ukazka.pdf"), []byte("není faktura"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := latestLocalDocumentNumber(root, "CZ9001010007")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("doklad mimo RRRR/MM nesmí vytvořit kotvu: %q", got)
	}
}

func TestLatestLocalDocumentUsesOnlyInvoicePeriods(t *testing.T) {
	root := t.TempDir()
	periodDir := filepath.Join(root, "2026", "08")
	if err := os.MkdirAll(periodDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Faktura-FV-2026-08-001.pdf", "Faktura-FV-2026-08-002.pdf"} {
		copyTestFile(t, filepath.Join("..", "..", "docs", "examples", name), filepath.Join(periodDir, name))
	}

	got, err := latestLocalDocumentNumber(root, "CZ9001010007")
	if err != nil {
		t.Fatal(err)
	}
	if got != "FV-2026-08-002" {
		t.Fatalf("poslední místní doklad = %q, chci FV-2026-08-002", got)
	}
}

func TestImportDownloadedPDFsRoutesByTaxableDate(t *testing.T) {
	root := t.TempDir()
	downloadDir := filepath.Join(root, ".dphcko", "idoklad-stazene")
	if err := os.MkdirAll(downloadDir, 0o700); err != nil {
		t.Fatal(err)
	}
	name := "Faktura-FV-2026-08-001.pdf"
	copyTestFile(t, filepath.Join("..", "..", "docs", "examples", name), filepath.Join(downloadDir, name))

	result, numbers, err := ImportDownloadedPDFs(downloadDir, root, "CZ9001010007")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(numbers, []string{"FV-2026-08-001"}) || len(result.Imported) != 1 {
		t.Fatalf("neočekávaný import: result=%#v, numbers=%#v", result, numbers)
	}
	if filepath.Dir(result.Imported[0]) != filepath.Join(root, "2026", "08") {
		t.Fatalf("faktura byla uložena do %q", result.Imported[0])
	}
}

func copyTestFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
