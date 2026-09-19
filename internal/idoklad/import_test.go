package idoklad

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImportPDFsDoesNotOverwriteExistingFiles(t *testing.T) {
	periodDir := t.TempDir()
	downloadDir := filepath.Join(periodDir, ".idoklad-stazene")
	if err := os.Mkdir(downloadDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(periodDir, "faktura.pdf"), []byte("původní"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(downloadDir, "faktura.pdf"), []byte("nová"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := ImportPDFs(downloadDir, periodDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Imported) != 1 || filepath.Base(result.Imported[0]) != "faktura-2.pdf" {
		t.Fatalf("neočekávaný import: %#v", result)
	}
	original, err := os.ReadFile(filepath.Join(periodDir, "faktura.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != "původní" {
		t.Fatalf("původní faktura byla přepsána: %q", original)
	}
}

func TestImportPDFsSkipsIdenticalFile(t *testing.T) {
	periodDir := t.TempDir()
	downloadDir := filepath.Join(periodDir, ".idoklad-stazene")
	if err := os.Mkdir(downloadDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(periodDir, "puvodni-nazev.pdf"), filepath.Join(downloadDir, "faktura.pdf")} {
		if err := os.WriteFile(path, []byte("stejná data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := ImportPDFs(downloadDir, periodDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Imported) != 0 || len(result.Skipped) != 1 {
		t.Fatalf("neočekávaný výsledek: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(downloadDir, "faktura.pdf")); !os.IsNotExist(err) {
		t.Fatalf("duplicitní soubor zůstal ve složce stahování: %v", err)
	}
}

func TestWaitForDownloads(t *testing.T) {
	dir := t.TempDir()
	pending := filepath.Join(dir, "faktura.pdf.crdownload")
	if err := os.WriteFile(pending, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WaitForDownloads(dir, 10*time.Millisecond); err == nil {
		t.Fatal("nedokončený download měl skončit chybou")
	}
	if err := os.Remove(pending); err != nil {
		t.Fatal(err)
	}
	if err := WaitForDownloads(dir, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForDownloadsIgnoresChromeComponentUpdates(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "downloads.html.crdownload"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WaitForDownloads(dir, time.Second); err != nil {
		t.Fatalf("ne-PDF stahování Chromu nemá blokovat import faktur: %v", err)
	}
}

func TestFindChromiumExecutable(t *testing.T) {
	got, err := findChromiumExecutable("linux", "chrome", func(name string) (string, error) {
		if name == "google-chrome-stable" {
			return "/usr/bin/google-chrome-stable", nil
		}
		return "", errors.New("nenalezeno")
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/usr/bin/google-chrome-stable" {
		t.Fatalf("nalezeno %q", got)
	}
}
