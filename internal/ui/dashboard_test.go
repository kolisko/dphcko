package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func TestPeriodWithGeneratedEPOOutputShowsGreenCheckAndOrangeSelection(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "2026", "08", "vystup")
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "DPHDP3_2026-08.xml"), []byte("<DPHDP3/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	periods := DiscoverPeriods(root)
	if len(periods) != 1 || !periods[0].HasEPOOutput {
		t.Fatalf("vygenerovaný výstup nebyl rozpoznán: %#v", periods)
	}
	m := dashboard{periods: periods}
	view := m.View().Content
	if !strings.Contains(view, lipgloss.NewStyle().Foreground(lipgloss.Color("#35BB78")).Render("✓")) {
		t.Fatalf("období nemá zelenou fajfku:\n%s", view)
	}
	orangeArrow := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5C451")).Render("<")
	if !strings.Contains(view, orangeArrow) {
		t.Fatalf("výběrový znak není oranžový:\n%s", view)
	}
	if strings.Contains(view, "›") {
		t.Fatalf("starý výběrový znak zůstal vlevo:\n%s", view)
	}
}

func TestEmptyEPOOutputDoesNotShowCompletion(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "vystup")
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "DPHDP3_2026-08.xml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if periodHasEPOOutput(dir, 2026, 8) {
		t.Fatal("prázdný XML soubor se nesmí tvářit jako hotový výstup")
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

func TestDashboardIShowsModalWithoutLeavingDashboard(t *testing.T) {
	m := dashboard{root: t.TempDir(), cfg: config.Config{Profile: config.Profile{VATID: "CZ9001010007"}}}

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'i', Text: "i"}))
	updated := next.(dashboard)
	if updated.modal == nil || updated.modal.kind != modalIDoklad || cmd == nil {
		t.Fatalf("i musí otevřít modální dialog: modal=%#v, cmd=%v", updated.modal, cmd)
	}
	view := updated.View().Content
	for _, expected := range []string{"Zatím není evidován žádný stažený doklad", "samostatné okno Chromu", "DPHČKO"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("modální obrazovka neobsahuje %q:\n%s", expected, view)
		}
	}
}

func TestDashboardNShowsNewPeriodModal(t *testing.T) {
	m := dashboard{root: t.TempDir()}

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	updated := next.(dashboard)
	if updated.modal == nil || updated.modal.kind != modalNewPeriod || cmd == nil {
		t.Fatalf("n musí otevřít modální formulář: modal=%#v, cmd=%v", updated.modal, cmd)
	}
}

func TestDashboardEscapeClosesModal(t *testing.T) {
	m := dashboard{root: t.TempDir()}
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	withModal := next.(dashboard)

	next, cmd := withModal.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	closed := next.(dashboard)
	if closed.modal != nil || cmd != nil {
		t.Fatalf("Esc musí zavřít dialog bez ukončení dashboardu: modal=%#v, cmd=%v", closed.modal, cmd)
	}
}

func TestDashboardGenerationShowsOverwriteModal(t *testing.T) {
	periodDir := t.TempDir()
	outputDir := filepath.Join(periodDir, "vystup")
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "DPHDP3_2026-08.xml"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := dashboard{periods: []Period{{Year: 2026, Month: time.August, Dir: periodDir}}}

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}))
	updated := next.(dashboard)
	if updated.modal == nil || updated.modal.kind != modalOverwrite || cmd == nil {
		t.Fatalf("existující výstup musí otevřít dialog přepsání: modal=%#v, cmd=%v", updated.modal, cmd)
	}
}

func TestDashboardCurrentMonthGenerationShowsWarningModal(t *testing.T) {
	now := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.Local)
	m := dashboard{periods: []Period{{Year: 2026, Month: time.September, Dir: t.TempDir()}}}

	next, cmd := m.prepareGenerationAt(now)
	updated := next.(dashboard)
	if updated.modal == nil || updated.modal.kind != modalCurrentPeriod || cmd == nil {
		t.Fatalf("aktuální měsíc musí otevřít varovný dialog: modal=%#v, cmd=%v", updated.modal, cmd)
	}
	view := updated.View().Content
	for _, expected := range []string{"ještě není ukončené", "Faktury přidané později", "Enter potvrdit"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("dialog neobsahuje %q:\n%s", expected, view)
		}
	}
	updated.modal.confirmed = true
	next, cmd = updated.completeModal()
	confirmed := next.(dashboard)
	if confirmed.action != ActionGenerate || cmd == nil {
		t.Fatalf("potvrzení aktuálního měsíce musí pokračovat: action=%v, cmd=%v", confirmed.action, cmd)
	}
}

func TestDashboardGenerationShowsConsumerModal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nadlimitni.pdf")
	m := dashboard{
		periods: []Period{{Year: 2026, Month: time.August, Dir: t.TempDir()}},
		results: []invoice.FileResult{{
			Path: path,
			Invoice: &invoice.Invoice{
				Number: "20260042", Total: invoice.A4Threshold + 1,
			},
			Err: errors.New("chybí DIČ odběratele"),
		}},
	}

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}))
	updated := next.(dashboard)
	if updated.modal == nil || updated.modal.kind != modalConsumer || cmd == nil {
		t.Fatalf("nadlimitní doklad musí otevřít dialog A.5: modal=%#v, cmd=%v", updated.modal, cmd)
	}
	updated.modal.confirmed = true
	next, cmd = updated.completeModal()
	confirmed := next.(dashboard)
	if !confirmed.generate.ConsumerDocuments[path] || confirmed.action != ActionGenerate || cmd == nil {
		t.Fatalf("potvrzení A.5 se nepředalo generátoru: options=%#v, action=%v, cmd=%v", confirmed.generate, confirmed.action, cmd)
	}
}

func TestDashboardUppercaseShortcutsWork(t *testing.T) {
	tests := []struct {
		key  rune
		want Action
	}{
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
	idokladPosition := strings.Index(menu, "iDoklad")
	reloadPosition := strings.Index(menu, "načíst")
	if newPosition < 0 || generatePosition < newPosition || epoPosition < generatePosition || idokladPosition < epoPosition || reloadPosition < idokladPosition {
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

func TestDashboardStatusLineHasFixedHeight(t *testing.T) {
	base := dashboard{cfg: config.Config{Profile: config.Profile{FirstName: "Jan", LastName: "Novák", VATID: "CZ9001010007"}}, width: 50}
	withoutNotice := strings.Split(base.View().Content, "\n")
	base.notice = "Vytvořeno: DPHDP3_2026-09.xml a další soubory, jejichž dlouhý název se nesmí zalomit."
	withNotice := strings.Split(base.View().Content, "\n")

	wantHeadingLine := -1
	for i, line := range withoutNotice {
		if strings.Contains(line, "Zdaňovací období") {
			wantHeadingLine = i
			break
		}
	}
	gotHeadingLine := -1
	for i, line := range withNotice {
		if strings.Contains(line, "Zdaňovací období") {
			gotHeadingLine = i
			break
		}
	}
	if wantHeadingLine < 0 || gotHeadingLine != wantHeadingLine {
		t.Fatalf("stavová zpráva posunula obsah: bez=%d, se zprávou=%d", wantHeadingLine, gotHeadingLine)
	}
	if strings.Count(withNotice[1], "\n") != 0 || !strings.Contains(withNotice[1], "…") {
		t.Fatalf("dlouhý stav nebyl udržen na jednom zkráceném řádku: %q", withNotice[1])
	}
}

func TestDashboardStatusClearsOnNextKeyPress(t *testing.T) {
	m := dashboard{
		notice:  "Vytvořeno.",
		periods: []Period{{Year: 2026, Month: time.August}, {Year: 2026, Month: time.September}},
	}

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated := next.(dashboard)
	if updated.notice != "" {
		t.Fatalf("stavová zpráva po stisku klávesy nezmizela: %q", updated.notice)
	}
	if updated.selected != 1 {
		t.Fatalf("klávesa se po skrytí zprávy neprovedla: selected=%d", updated.selected)
	}
}

func TestDashboardReloadReplacesStatusMessage(t *testing.T) {
	m := dashboard{notice: "Původní zpráva."}

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	updated := next.(dashboard)
	if updated.notice != "Složka období byla znovu načtena." {
		t.Fatalf("R má původní stav nahradit novým: %q", updated.notice)
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
