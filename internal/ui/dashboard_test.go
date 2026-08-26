package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"dphcko/internal/config"
	"dphcko/internal/invoice"
)

func TestDashboardSummaryMatchesA4AndA5(t *testing.T) {
	date := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	results := []invoice.FileResult{
		{Path: "Faktura-FV-2026-08-001.pdf", Invoice: &invoice.Invoice{Number: "FV-2026-08-001", TaxableDate: date, TaxBase: 100000, Tax: 21000, Total: 121000}},
		{Path: "Faktura-FV-2026-08-002.pdf", Invoice: &invoice.Invoice{Number: "FV-2026-08-002", RecipientVATID: "CZ27082440", TaxableDate: date, TaxBase: 1000000, Tax: 210000, Total: 1210000}},
	}

	summary, invalid, err := summarizeResults(results)
	if err != nil {
		t.Fatal(err)
	}
	if invalid != 0 || len(summary.A4) != 1 || len(summary.A5) != 1 {
		t.Fatalf("neočekávané rozdělení: invalid=%d, A.4=%d, A.5=%d", invalid, len(summary.A4), len(summary.A5))
	}
	if summary.Base != 1100000 || summary.Tax != 231000 || summary.Total != 1331000 {
		t.Fatalf("neočekávané součty: %#v", summary)
	}
}

func TestDashboardViewShowsTaxSummaryBeforeGeneration(t *testing.T) {
	date := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	m := dashboard{
		cfg:     config.Config{Profile: config.Profile{FirstName: "Jan", LastName: "Novák", VATID: "CZ9001010007"}},
		periods: []Period{{Year: 2026, Month: time.August}},
		results: []invoice.FileResult{
			{Path: "Faktura-FV-2026-08-001.pdf", Invoice: &invoice.Invoice{Number: "FV-2026-08-001", TaxableDate: date, TaxBase: 100000, Tax: 21000, Total: 121000}},
			{Path: "Faktura-FV-2026-08-002.pdf", Invoice: &invoice.Invoice{Number: "FV-2026-08-002", RecipientVATID: "CZ27082440", TaxableDate: date, TaxBase: 1000000, Tax: 210000, Total: 1210000}},
		},
	}

	view := m.View().Content
	for _, expected := range []string{
		"Souhrn DPH z platných faktur",
		"Platné doklady: 2 · A.4: 1 · A.5: 1",
		"11 000,00 Kč",
		"2 310,00 Kč",
		"13 310,00 Kč",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("obrazovka neobsahuje %q:\n%s", expected, view)
		}
	}
}

func TestDashboardSummaryWarnsAboutInvalidInvoices(t *testing.T) {
	m := dashboard{
		results: []invoice.FileResult{{Path: "vadna.pdf", Err: errors.New("chybí QR Faktura")}},
	}
	view := m.View().Content
	if !strings.Contains(view, "Generování je zablokované. Chybné faktury: 1") {
		t.Fatalf("chybí upozornění na blokované generování:\n%s", view)
	}
}

func TestFormatCZK(t *testing.T) {
	for value, want := range map[invoice.Money]string{
		0:        "0,00 Kč",
		121000:   "1 210,00 Kč",
		1331000:  "13 310,00 Kč",
		-2100500: "-21 005,00 Kč",
	} {
		if got := formatCZK(value); got != want {
			t.Errorf("formatCZK(%d) = %q, chci %q", value, got, want)
		}
	}
}
