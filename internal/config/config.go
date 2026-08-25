package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	FileName      = "dphcko.toml"
	CurrentFormat = 1
)

type Config struct {
	Format  int     `toml:"format"`
	Profile Profile `toml:"profile"`
}

type Profile struct {
	ICO             string `toml:"ico"`
	VATID           string `toml:"dic"`
	FirstName       string `toml:"jmeno"`
	LastName        string `toml:"prijmeni"`
	Street          string `toml:"ulice"`
	HouseNumber     string `toml:"cislo_popisne"`
	OrientationNo   string `toml:"cislo_orientacni,omitempty"`
	City            string `toml:"obec"`
	PostalCode      string `toml:"psc"`
	Country         string `toml:"stat"`
	TaxOffice       string `toml:"financni_urad"`
	TaxOfficeBranch string `toml:"uzemni_pracoviste"`
	NACE            string `toml:"cz_nace"`
	Phone           string `toml:"telefon"`
	Email           string `toml:"email"`
}

func Default() Config {
	return Config{Format: CurrentFormat, Profile: Profile{Country: "CZ"}}
}

func Path(root string) string { return filepath.Join(root, FileName) }

func Load(root string) (Config, error) {
	data, err := os.ReadFile(Path(root))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("načtení %s: %w", FileName, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("neplatná konfigurace: %w", err)
	}
	return cfg, nil
}

func Exists(root string) bool {
	_, err := os.Stat(Path(root))
	return err == nil
}

func Save(root string, cfg Config) error {
	cfg.Format = CurrentFormat
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	path := Path(root)
	tmp, err := os.CreateTemp(root, ".dphcko-*.toml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

func (c Config) Validate() error {
	p := c.Profile
	checks := []struct {
		name  string
		value string
	}{
		{"IČO", p.ICO}, {"DIČ", p.VATID}, {"jméno", p.FirstName}, {"příjmení", p.LastName},
		{"ulice", p.Street}, {"číslo popisné", p.HouseNumber}, {"obec", p.City}, {"PSČ", p.PostalCode},
		{"finanční úřad", p.TaxOffice}, {"územní pracoviště", p.TaxOfficeBranch}, {"CZ-NACE", p.NACE},
		{"telefon", p.Phone}, {"e-mail", p.Email},
	}
	for _, check := range checks {
		if strings.TrimSpace(check.value) == "" {
			return fmt.Errorf("chybí %s", check.name)
		}
	}
	if !regexp.MustCompile(`^\d{8}$`).MatchString(p.ICO) {
		return errors.New("IČO musí mít 8 číslic")
	}
	if !regexp.MustCompile(`^CZ[0-9]{8,10}$`).MatchString(strings.ToUpper(strings.ReplaceAll(p.VATID, " ", ""))) {
		return errors.New("DIČ musí mít tvar CZ a 8 až 10 číslic")
	}
	if !regexp.MustCompile(`^\d{5}$`).MatchString(strings.ReplaceAll(p.PostalCode, " ", "")) {
		return errors.New("PSČ musí mít 5 číslic")
	}
	if !regexp.MustCompile(`^\d{3}$`).MatchString(p.TaxOffice) {
		return errors.New("kód finančního úřadu musí mít 3 číslice")
	}
	if !regexp.MustCompile(`^\d{4}$`).MatchString(p.TaxOfficeBranch) {
		return errors.New("kód územního pracoviště musí mít 4 číslice")
	}
	if !regexp.MustCompile(`^\d{1,6}$`).MatchString(p.NACE) {
		return errors.New("CZ-NACE musí mít 1 až 6 číslic")
	}
	if !regexp.MustCompile(`^\d{1,6}$`).MatchString(p.HouseNumber) {
		return errors.New("číslo popisné musí mít 1 až 6 číslic")
	}
	if len(p.OrientationNo) > 4 {
		return errors.New("číslo orientační může mít nejvýše 4 znaky")
	}
	if len([]rune(p.FirstName)) > 20 || len([]rune(p.LastName)) > 36 {
		return errors.New("jméno může mít nejvýše 20 a příjmení 36 znaků")
	}
	if len([]rune(p.Street)) > 38 || len([]rune(p.City)) > 48 {
		return errors.New("ulice může mít nejvýše 38 a obec 48 znaků")
	}
	if len(p.Phone) > 14 {
		return errors.New("telefon může mít nejvýše 14 znaků")
	}
	if len(p.Email) > 255 || !strings.Contains(p.Email, "@") {
		return errors.New("e-mail není platný")
	}
	if p.Country != "CZ" {
		return errors.New("v1 podporuje pouze stát CZ")
	}
	return nil
}
