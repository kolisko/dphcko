package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"dphcko/internal/config"
	"dphcko/internal/invoice"
)

func TestDiscoverPeriodsShowsNewestAtBottomAndSelectsIt(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"2027/01", "2026/12", "2026/07"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}

	periods := DiscoverPeriods(root)
	want := []string{"2026/07", "2026/12", "2027/01"}
	if len(periods) != len(want) {
		t.Fatalf("počet období = %d, chci %d", len(periods), len(want))
	}
	for i, expected := range want {
		if got := periods[i].String(); got != expected {
			t.Fatalf("období[%d] = %s, chci %s", i, got, expected)
		}
	}
	if selected := newestPeriodIndex(periods); selected != len(periods)-1 {
		t.Fatalf("výchozí výběr = %d, chci poslední index %d", selected, len(periods)-1)
	}
}

func TestDashboardEnterDoesNotGenerate(t *testing.T) {
	m := dashboard{periods: []Period{{Year: 2026, Month: time.August}}}

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	updated := next.(dashboard)
	if updated.action == ActionGenerate {
		t.Fatal("Enter nesmí spustit generování")
	}
	if cmd != nil {
		t.Fatal("Enter nesmí ukončit dashboard")
	}
	if updated.notice != "" {
		t.Fatalf("Enter nemá měnit stav dashboardu: %q", updated.notice)
	}
}

func TestDashboardGStartsGeneration(t *testing.T) {
	m := dashboard{periods: []Period{{Year: 2026, Month: time.August}}}

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}))
	updated := next.(dashboard)
	if updated.action != ActionGenerate || cmd == nil {
		t.Fatalf("g musí spustit generování: action=%v, cmd=%v", updated.action, cmd)
	}
}

func TestDashboardUppercaseShortcutsWork(t *testing.T) {
	tests := []struct {
		key  rune
		want Action
	}{
		{key: 'N', want: ActionNewPeriod},
		{key: 'G', want: ActionGenerate},
		{key: 'O', want: ActionOpenEPO},
	}
	for _, test := range tests {
		m := dashboard{periods: []Period{{Year: 2026, Month: time.August}}}
		next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: test.key, Text: string(test.key)}))
		updated := next.(dashboard)
		if updated.action != test.want || cmd == nil {
			t.Errorf("klávesa %c: action=%v, cmd=%v; chci action=%v a ukončení dashboardu", test.key, updated.action, cmd, test.want)
		}
	}
}

func TestDashboardMenuStartsWithNewPeriodThenGenerateAndEPO(t *testing.T) {
	menu := renderDashboardMenu(false)
	newPosition := strings.Index(menu, "nové období")
	generatePosition := strings.Index(menu, "generovat")
	epoPosition := strings.Index(menu, "otevřít EPO")
	reloadPosition := strings.Index(menu, "načíst")
	if newPosition < 0 || generatePosition < newPosition || epoPosition < generatePosition || reloadPosition < epoPosition {
		t.Fatalf("neočekávané pořadí menu: %q", menu)
	}
	if !strings.Contains(menu, "\x1b[") {
		t.Fatalf("menu nemá barevný styl: %q", menu)
	}
}

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
	if strings.Contains(view, "Enter/g generovat") {
		t.Fatalf("nápověda stále nabízí generování Enterem:\n%s", view)
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
