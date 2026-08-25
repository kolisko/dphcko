package ui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"charm.land/huh/v2"
	"dphcko/internal/config"
)

func RunProfileWizard(root string, existing *config.Config) (config.Config, error) {
	cfg := config.Default()
	if existing != nil {
		cfg = *existing
	}
	ico := cfg.Profile.ICO
	if err := huh.NewInput().
		Title("IČO plátce").
		Description("ARES použijeme pouze k předvyplnění veřejných údajů.").
		Value(&ico).
		Validate(func(value string) error {
			if !regexp.MustCompile(`^\d{8}$`).MatchString(strings.TrimSpace(value)) {
				return errors.New("IČO musí mít 8 číslic")
			}
			return nil
		}).Run(); err != nil {
		return config.Config{}, err
	}

	if found, err := config.LookupARES(context.Background(), strings.TrimSpace(ico)); err == nil {
		if existing == nil || cfg.Profile.ICO != ico {
			cfg.Profile = found
		} else {
			mergeMissing(&cfg.Profile, found)
		}
	} else {
		fmt.Printf("ARES se nepodařilo načíst (%v). Údaje vyplňte ručně.\n\n", err)
		cfg.Profile.ICO = ico
	}
	cfg.Profile.ICO = ico
	cfg.Profile.Country = "CZ"
	p := &cfg.Profile
	required := func(label string) func(string) error {
		return func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("pole %s je povinné", label)
			}
			return nil
		}
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("DIČ").Value(&p.VATID).Validate(func(value string) error {
				value = strings.ToUpper(strings.ReplaceAll(value, " ", ""))
				if !regexp.MustCompile(`^CZ[0-9]{8,10}$`).MatchString(value) {
					return errors.New("očekávám CZ a 8 až 10 číslic")
				}
				return nil
			}),
			huh.NewInput().Title("Jméno").Value(&p.FirstName).Validate(required("jméno")),
			huh.NewInput().Title("Příjmení").Value(&p.LastName).Validate(required("příjmení")),
		),
		huh.NewGroup(
			huh.NewInput().Title("Ulice nebo část obce").Value(&p.Street).Validate(required("ulice")),
			huh.NewInput().Title("Číslo popisné").Value(&p.HouseNumber).Validate(required("číslo popisné")),
			huh.NewInput().Title("Číslo orientační (nepovinné)").Value(&p.OrientationNo),
			huh.NewInput().Title("Obec").Value(&p.City).Validate(required("obec")),
			huh.NewInput().Title("PSČ").Value(&p.PostalCode).Validate(func(value string) error {
				if !regexp.MustCompile(`^\d{3}\s?\d{2}$`).MatchString(value) {
					return errors.New("PSČ musí mít 5 číslic")
				}
				return nil
			}),
		),
		huh.NewGroup(
			huh.NewInput().Title("Kód finančního úřadu (3 číslice)").Value(&p.TaxOffice).Validate(numericLength("finanční úřad", 3)),
			huh.NewInput().Title("Kód územního pracoviště (4 číslice)").Value(&p.TaxOfficeBranch).Validate(numericLength("územní pracoviště", 4)),
			huh.NewInput().Title("CZ-NACE (max. 6 číslic)").Value(&p.NACE).Validate(func(value string) error {
				if !regexp.MustCompile(`^\d{1,6}$`).MatchString(value) {
					return errors.New("CZ-NACE musí obsahovat 1 až 6 číslic")
				}
				return nil
			}),
		),
		huh.NewGroup(
			huh.NewInput().Title("Telefon").Value(&p.Phone).Validate(func(value string) error {
				if len(value) > 14 || strings.TrimSpace(value) == "" {
					return errors.New("telefon je povinný a může mít nejvýše 14 znaků")
				}
				return nil
			}),
			huh.NewInput().Title("E-mail").Value(&p.Email).Validate(func(value string) error {
				if !strings.Contains(value, "@") || len(value) > 255 {
					return errors.New("zadejte platnou e-mailovou adresu")
				}
				return nil
			}),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeCharm))
	if err := form.Run(); err != nil {
		return config.Config{}, err
	}
	p.VATID = strings.ToUpper(strings.ReplaceAll(p.VATID, " ", ""))
	p.PostalCode = strings.ReplaceAll(p.PostalCode, " ", "")
	if err := config.Save(root, cfg); err != nil {
		return config.Config{}, fmt.Errorf("uložení konfigurace: %w", err)
	}
	return cfg, nil
}

func numericLength(label string, length int) func(string) error {
	return func(value string) error {
		if !regexp.MustCompile(fmt.Sprintf(`^\d{%d}$`, length)).MatchString(value) {
			return fmt.Errorf("%s musí mít %d číslice", label, length)
		}
		return nil
	}
}

func mergeMissing(dst *config.Profile, src config.Profile) {
	if dst.VATID == "" {
		dst.VATID = src.VATID
	}
	if dst.FirstName == "" {
		dst.FirstName = src.FirstName
	}
	if dst.LastName == "" {
		dst.LastName = src.LastName
	}
	if dst.Street == "" {
		dst.Street = src.Street
	}
	if dst.HouseNumber == "" {
		dst.HouseNumber = src.HouseNumber
	}
	if dst.OrientationNo == "" {
		dst.OrientationNo = src.OrientationNo
	}
	if dst.City == "" {
		dst.City = src.City
	}
	if dst.PostalCode == "" {
		dst.PostalCode = src.PostalCode
	}
	if dst.TaxOffice == "" {
		dst.TaxOffice = src.TaxOffice
	}
	if dst.NACE == "" {
		dst.NACE = src.NACE
	}
}
