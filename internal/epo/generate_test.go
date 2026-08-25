package epo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dphcko/internal/config"
	"dphcko/internal/invoice"
	"dphcko/internal/tax"
)

func testProfile() config.Profile {
	return config.Profile{
		ICO: "27074358", VATID: "CZ9001010007", FirstName: "Jan", LastName: "Novák",
		Street: "Dlouhá", HouseNumber: "1", OrientationNo: "2", City: "Praha", PostalCode: "11000", Country: "CZ",
		TaxOffice: "451", TaxOfficeBranch: "2001", NACE: "62010", Phone: "+420123456789", Email: "jan@example.cz",
	}
}

func testSummary(t *testing.T) tax.Summary {
	t.Helper()
	date := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	summary, err := tax.Build([]invoice.Invoice{
		{Number: "FV-A5", TaxableDate: date, TaxBase: 100000, Tax: 21000, Total: 121000},
		{Number: "FV-A4", RecipientVATID: "CZ27082440", TaxableDate: date, TaxBase: 1000000, Tax: 210000, Total: 1210000},
	})
	if err != nil {
		t.Fatal(err)
	}
	return summary
}

func TestDPHGolden(t *testing.T) {
	got, err := DPH(testProfile(), 2026, 8, testSummary(t), time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(`stat="ČESKÁ REPUBLIKA"`)) || bytes.Contains(got, []byte(`stat="CZ"`)) {
		t.Fatalf("stát musí být název z číselníku EPO, nikoli ISO kód:\n%s", got)
	}
	assertGolden(t, "dphdp3.golden.xml", got)
}

func TestKHGoldenAndControlTotals(t *testing.T) {
	summary := testSummary(t)
	if summary.A5Base+summary.A4[0].TaxBase != summary.Base || summary.A5Tax+summary.A4[0].Tax != summary.Tax {
		t.Fatal("A.4 + A.5 se nerovná kontrolnímu součtu")
	}
	got, err := KH(testProfile(), 2026, 8, summary, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "dphkh1.golden.xml", got)
}

func TestZeroDPHAndNoKH(t *testing.T) {
	got, err := DPH(testProfile(), 2026, 8, tax.Summary{}, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("<Veta1")) || !bytes.Contains(got, []byte(`trans="N"`)) {
		t.Fatalf("neočekávané nulové přiznání:\n%s", got)
	}
	if _, err := KH(testProfile(), 2026, 8, tax.Summary{}, time.Now()); err == nil {
		t.Fatal("prázdné KH se nemá generovat")
	}
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("XML neodpovídá golden souboru\n--- chci ---\n%s\n--- mám ---\n%s", want, got)
	}
}
