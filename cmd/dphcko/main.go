package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/huh/v2"
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
		action, period, err := ui.RunDashboard(root, cfg, notice)
		if err != nil {
			fatal(err)
		}
		notice = ""
		switch action {
		case ui.ActionQuit:
			return
		case ui.ActionNewPeriod:
			created, err := ui.CreatePeriod(root, time.Now())
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				notice = "Chyba: " + err.Error()
				continue
			}
			notice = "Založena složka " + created.String() + ". Vložte do ní PDF faktury."
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
			if err := epo.OpenUploadPage(); err != nil {
				notice = "Stránku EPO se nepodařilo otevřít: " + err.Error() + "."
			} else {
				notice = "V prohlížeči byla otevřena stránka EPO pro ruční načtení XML."
			}
		case ui.ActionGenerate:
			if period == nil {
				continue
			}
			paths, err := ui.Generate(root, cfg, *period, time.Now())
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
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
			err = epo.OpenUploadPage()
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
