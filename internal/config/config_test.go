package config

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func validConfig() Config {
	return Config{Format: CurrentFormat, Profile: Profile{
		ICO: "12345678", VATID: "CZ12345678", FirstName: "Jan", LastName: "Novák",
		Street: "Dlouhá", HouseNumber: "1", City: "Praha", PostalCode: "11000", Country: "CZ",
		TaxOffice: "200", TaxOfficeBranch: "2001", NACE: "62010", Phone: "+420123456789", Email: "jan@example.cz",
	}}
}

func TestSaveLoadAndPermissions(t *testing.T) {
	dir := t.TempDir()
	want := validConfig()
	if err := Save(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("načtená konfigurace = %#v, chci %#v", got, want)
	}
	info, err := os.Stat(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("oprávnění konfigurace = %o", info.Mode().Perm())
	}
}

func TestValidateRejectsSchemaInvalidProfile(t *testing.T) {
	cfg := validConfig()
	cfg.Profile.TaxOffice = "20"
	if err := cfg.Validate(); err == nil {
		t.Fatal("neplatný kód finančního úřadu měl být odmítnut")
	}
	cfg = validConfig()
	cfg.Profile.Street = strings.Repeat("x", 39)
	if err := cfg.Validate(); err == nil {
		t.Fatal("příliš dlouhá ulice měla být odmítnuta")
	}
}

func TestLookupARES(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://ares.test/12345678" {
			t.Fatalf("neočekávaná URL %s", req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"ico":"12345678","obchodniJmeno":"Jan Novák","dic":"CZ12345678","financniUrad":"004","czNace":["62010"],"sidlo":{"nazevUlice":"Dlouhá","nazevObce":"Praha","nazevKraje":"Hlavní město Praha","cisloDomovni":1,"cisloOrientacni":2,"cisloOrientacniPismeno":"a","psc":11000}}`)),
		}, nil
	})}
	profile, err := lookupARES(context.Background(), client, "https://ares.test/", "12345678")
	if err != nil {
		t.Fatal(err)
	}
	if profile.FirstName != "Jan" || profile.LastName != "Novák" || profile.OrientationNo != "2a" || profile.NACE != "62010" || profile.TaxOffice != "451" {
		t.Fatalf("neočekávaný profil: %#v", profile)
	}
}

func TestEPOOfficeMapping(t *testing.T) {
	tests := map[string]string{
		"Hlavní město Praha": "451", "Středočeský kraj": "452", "Jihočeský kraj": "453",
		"Plzeňský kraj": "454", "Karlovarský kraj": "455", "Ústecký kraj": "456",
		"Liberecký kraj": "457", "Královéhradecký kraj": "458", "Pardubický kraj": "459",
		"Kraj Vysočina": "460", "Jihomoravský kraj": "461", "Olomoucký kraj": "462",
		"Moravskoslezský kraj": "463", "Zlínský kraj": "464",
	}
	for region, want := range tests {
		if got := epoOfficeForRegion(region); got != want {
			t.Errorf("%s: dostal jsem %s, chci %s", region, got, want)
		}
	}
}

func TestLookupARESOffline(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}
	if _, err := lookupARES(context.Background(), client, "https://ares.test/", "12345678"); err == nil {
		t.Fatal("nedostupný ARES měl vrátit chybu pro ruční režim")
	}
}
