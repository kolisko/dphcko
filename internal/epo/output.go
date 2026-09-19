package epo

import (
	"errors"
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

var ErrOutputExists = errors.New("výstupní soubory už existují")

func WritePeriod(periodDir string, profile config.Profile, year, month int, summary tax.Summary, now time.Time) (OutputPaths, error) {
	return writePeriod(periodDir, profile, year, month, summary, now, false)
}

func ReplacePeriod(periodDir string, profile config.Profile, year, month int, summary tax.Summary, now time.Time) (OutputPaths, error) {
	return writePeriod(periodDir, profile, year, month, summary, now, true)
}

func ExistingPeriodOutputs(periodDir string, year, month int) ([]string, error) {
	paths := periodOutputPaths(periodDir, year, month)
	candidates := []string{paths.DPH, paths.KH, paths.Summary}
	existing := make([]string, 0, len(candidates))
	for _, path := range candidates {
		_, err := os.Stat(path)
		switch {
		case err == nil:
			existing = append(existing, path)
		case os.IsNotExist(err):
			continue
		default:
			return nil, fmt.Errorf("kontrola existujícího výstupu %s: %w", filepath.Base(path), err)
		}
	}
	return existing, nil
}

func writePeriod(periodDir string, profile config.Profile, year, month int, summary tax.Summary, now time.Time, overwrite bool) (OutputPaths, error) {
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
	allPaths := periodOutputPaths(periodDir, year, month)
	if !overwrite {
		existing, err := ExistingPeriodOutputs(periodDir, year, month)
		if err != nil {
			return OutputPaths{}, err
		}
		if len(existing) > 0 {
			names := make([]string, len(existing))
			for i, path := range existing {
				names[i] = filepath.Base(path)
			}
			return OutputPaths{}, fmt.Errorf("%w: %s", ErrOutputExists, strings.Join(names, ", "))
		}
	}
	paths := allPaths
	if len(khData) == 0 {
		paths.KH = ""
	}
	staleKH := allPaths.KH
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

func periodOutputPaths(periodDir string, year, month int) OutputPaths {
	outDir := filepath.Join(periodDir, "vystup")
	suffix := fmt.Sprintf("%04d-%02d", year, month)
	return OutputPaths{
		DPH:     filepath.Join(outDir, "DPHDP3_"+suffix+".xml"),
		KH:      filepath.Join(outDir, "DPHKH1_"+suffix+".xml"),
		Summary: filepath.Join(outDir, "prehled_"+suffix+".txt"),
	}
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
