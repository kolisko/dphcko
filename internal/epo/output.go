package epo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dphcko/internal/config"
	"dphcko/internal/tax"
)

type OutputPaths struct {
	DPH     string
	KH      string
	Summary string
}

func WritePeriod(periodDir string, profile config.Profile, year, month int, summary tax.Summary, now time.Time) (OutputPaths, error) {
	dphData, err := DPH(profile, year, month, summary, now)
	if err != nil {
		return OutputPaths{}, err
	}
	var khData []byte
	if len(summary.Invoices) > 0 {
		khData, err = KH(profile, year, month, summary, now)
		if err != nil {
			return OutputPaths{}, err
		}
	}
	outDir := filepath.Join(periodDir, "vystup")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return OutputPaths{}, err
	}
	suffix := fmt.Sprintf("%04d-%02d", year, month)
	paths := OutputPaths{
		DPH:     filepath.Join(outDir, "DPHDP3_"+suffix+".xml"),
		Summary: filepath.Join(outDir, "prehled_"+suffix+".txt"),
	}
	if len(khData) > 0 {
		paths.KH = filepath.Join(outDir, "DPHKH1_"+suffix+".xml")
	}
	staleKH := filepath.Join(outDir, "DPHKH1_"+suffix+".xml")
	report := textSummary(profile, year, month, summary, now)
	files := []struct {
		path string
		data []byte
	}{
		{paths.DPH, dphData},
		{paths.KH, khData},
		{paths.Summary, []byte(report)},
	}
	for _, file := range files {
		path, data := file.path, file.data
		if path == "" {
			continue
		}
		if err := atomicWrite(path, data, 0o600); err != nil {
			return OutputPaths{}, err
		}
	}
	if paths.KH == "" {
		if err := os.Remove(staleKH); err != nil && !os.IsNotExist(err) {
			return OutputPaths{}, fmt.Errorf("odstranění starého kontrolního hlášení: %w", err)
		}
	}
	return paths, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".dphcko-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func textSummary(profile config.Profile, year, month int, summary tax.Summary, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DPHČKO – kontrolní přehled %04d/%02d\n", year, month)
	fmt.Fprintf(&b, "Vytvořeno: %s\nPlátce: %s %s, DIČ %s\n\n", now.Format("02.01.2006 15:04"), profile.FirstName, profile.LastName, profile.VATID)
	fmt.Fprintf(&b, "Počet faktur: %d (A.4: %d, A.5: %d)\n", len(summary.Invoices), len(summary.A4), len(summary.A5))
	if len(summary.Invoices) == 0 {
		b.WriteString("Kontrolní hlášení nebylo vytvořeno: v podporovaném rozsahu nejsou žádné doklady.\n")
	}
	fmt.Fprintf(&b, "Základ DPH: %s Kč\nDPH 21 %%: %s Kč\nCelkem: %s Kč\n", summary.Base.String(), summary.Tax.String(), summary.Total.String())
	fmt.Fprintf(&b, "Řádek 1 přiznání po zaokrouhlení: základ %d Kč, daň %d Kč\n\n", summary.Base.WholeCrowns(), summary.Tax.WholeCrowns())
	for _, inv := range summary.Invoices {
		fmt.Fprintf(&b, "%s  %s  %s Kč  %s\n", inv.TaxableDate.Format("02.01.2006"), inv.Number, inv.Total.String(), inv.SourcePath)
	}
	b.WriteString("\nPOZOR: Výstup pokrývá pouze běžné tuzemské vydané faktury v CZK se sazbou 21 %, bez odpočtů. Před podáním proveďte kontrolu v EPO.\n")
	return b.String()
}
