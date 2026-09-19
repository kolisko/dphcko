package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dphcko/internal/config"
	"dphcko/internal/epo"
	"dphcko/internal/ui"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("dphcko %s\n", version)
		return
	}
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	cfg, err := loadOrCreateProfile(root)
	if err != nil {
		fatal(err)
	}

	notice := ""
	for {
		action, period, generateOptions, err := ui.RunDashboard(root, cfg, notice)
		if err != nil {
			fatal(err)
		}
		notice = ""
		switch action {
		case ui.ActionQuit:
			return
		case ui.ActionConfig:
			updated, saved, err := ui.RunProfileEditor(root, cfg)
			if err != nil {
				notice = "Chyba konfigurace: " + err.Error()
				continue
			}
			if !saved {
				continue
			}
			cfg = updated
			notice = "Profil byl uložen do " + config.FileName + "."
		case ui.ActionOpenEPO:
			if err := epo.OpenUploadPage(cfg.App.Browser); err != nil {
				notice = "Stránku EPO se nepodařilo otevřít: " + err.Error() + "."
			} else {
				notice = "V prohlížeči byla otevřena stránka EPO pro ruční načtení XML."
			}
		case ui.ActionImportIDoklad:
			result, err := ui.ImportFromIDoklad(root, cfg)
			if err != nil {
				notice = "Synchronizace iDokladu se nepodařila: " + err.Error()
				continue
			}
			if len(result.RemoteDocuments) == 0 {
				notice = "iDoklad je synchronizovaný; žádné novější faktury nebyly nalezeny."
				continue
			}
			notice = fmt.Sprintf("iDoklad: staženo %d nových PDF, %d už bylo místně. Poslední doklad je %s.", len(result.Imported), len(result.Skipped), result.NewestDocumentNumber)
		case ui.ActionGenerate:
			if period == nil {
				continue
			}
			paths, err := ui.Generate(root, cfg, *period, time.Now(), generateOptions)
			if err != nil {
				notice = "Generování zastaveno: " + err.Error()
				continue
			}
			files := []string{filepath.Base(paths.DPH)}
			if paths.KH != "" {
				files = append(files, filepath.Base(paths.KH))
			}
			files = append(files, filepath.Base(paths.Summary))
			notice = "Vytvořeno: " + strings.Join(files, ", ") + "."
			if paths.KH == "" {
				notice += " Kontrolní hlášení nevzniklo, protože v podporovaném rozsahu nejsou žádné doklady."
			}
			err = epo.OpenUploadPage(cfg.App.Browser)
			if err != nil {
				notice += " XML jsou uložená, ale stránku EPO se nepodařilo otevřít: " + err.Error() + "."
			} else {
				notice += " V prohlížeči byla otevřena stránka EPO pro ruční načtení XML."
			}
		}
	}
}

func loadOrCreateProfile(root string) (config.Config, error) {
	if config.Exists(root) {
		return config.Load(root)
	}
	fmt.Println("Vítejte v DPHČKU. Nejdřív vytvoříme místní profil plátce.")
	return ui.RunProfileWizard(root, nil)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "dphcko:", err)
	os.Exit(1)
}
