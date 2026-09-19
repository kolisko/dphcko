package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"dphcko/internal/config"
	"dphcko/internal/epo"
	"dphcko/internal/idoklad"
	"dphcko/internal/invoice"
	"dphcko/internal/tax"
	"github.com/charmbracelet/x/ansi"
)

type Action int

const (
	ActionQuit Action = iota
	ActionGenerate
	ActionConfig
	ActionOpenEPO
	ActionImportIDoklad
)

type Period struct {
	Year         int
	Month        time.Month
	Dir          string
	HasEPOOutput bool
}

func (p Period) String() string { return fmt.Sprintf("%04d/%02d", p.Year, p.Month) }

type dashboard struct {
	root     string
	cfg      config.Config
	periods  []Period
	selected int
	results  []invoice.FileResult
	action   Action
	help     bool
	width    int
	height   int
	notice   string
	modal    *dashboardModal
	pending  []modalRequest
	generate GenerateOptions
}

type modalKind int

const (
	modalNewPeriod modalKind = iota
	modalIDoklad
	modalCurrentPeriod
	modalConsumer
	modalOverwrite
)

type modalRequest struct {
	kind        modalKind
	title       string
	description string
	affirmative string
	negative    string
	path        string
}

type dashboardModal struct {
	kind       modalKind
	form       *huh.Form
	help       string
	confirmed  bool
	yearValue  string
	monthValue string
	path       string
}

func RunDashboard(root string, cfg config.Config, notice string) (Action, *Period, GenerateOptions, error) {
	periods := DiscoverPeriods(root)
	m := dashboard{root: root, cfg: cfg, periods: periods, selected: newestPeriodIndex(periods), notice: notice}
	m.reload()
	program := tea.NewProgram(m)
	final, err := program.Run()
	if err != nil {
		return ActionQuit, nil, GenerateOptions{}, err
	}
	result := final.(dashboard)
	if len(result.periods) == 0 {
		return result.action, nil, result.generate, nil
	}
	period := result.periods[result.selected]
	return result.action, &period, result.generate, nil
}

func newestPeriodIndex(periods []Period) int {
	if len(periods) == 0 {
		return 0
	}
	return len(periods) - 1
}

func (m dashboard) Init() tea.Cmd { return nil }

func (m dashboard) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := message.(tea.KeyPressMsg); ok {
		if msg.String() == "ctrl+c" {
			m.action = ActionQuit
			return m, tea.Quit
		}
		m.notice = ""
	}
	if m.modal != nil {
		return m.updateModal(message)
	}
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "Q":
			m.action = ActionQuit
			return m, tea.Quit
		case "n", "N":
			return m.openNewPeriodModal()
		case "c", "C":
			m.action = ActionConfig
			return m, tea.Quit
		case "o", "O":
			m.action = ActionOpenEPO
			return m, tea.Quit
		case "i", "I":
			return m.openIDokladModal()
		case "g", "G":
			if len(m.periods) > 0 {
				return m.prepareGenerationAt(time.Now())
			}
		case "r", "R":
			m.reload()
			m.notice = "Složka období byla znovu načtena."
		case "?":
			m.help = !m.help
		case "up", "k", "K":
			if m.selected > 0 {
				m.selected--
				m.reload()
			}
		case "down", "j", "J":
			if m.selected+1 < len(m.periods) {
				m.selected++
				m.reload()
			}
		}
	}
	return m, nil
}

func (m dashboard) openNewPeriodModal() (tea.Model, tea.Cmd) {
	year, month := invoice.PreviousMonth(time.Now())
	modal := &dashboardModal{
		kind:       modalNewPeriod,
		help:       "Tab/Shift+Tab změnit pole · Enter pokračovat · Esc zavřít",
		yearValue:  strconv.Itoa(year),
		monthValue: fmt.Sprintf("%02d", month),
	}
	modal.form = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Rok").Value(&modal.yearValue).Validate(func(value string) error {
			n, err := strconv.Atoi(value)
			if err != nil || n < 2000 || n > 2100 {
				return errors.New("zadejte rok 2000 až 2100")
			}
			return nil
		}),
		huh.NewInput().Title("Měsíc").Description("Předvolený je poslední dokončený měsíc.").Value(&modal.monthValue).Validate(func(value string) error {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 12 {
				return errors.New("zadejte měsíc 1 až 12")
			}
			return nil
		}),
	)).WithWidth(m.modalFormWidth(64)).WithShowHelp(false)
	m.modal = modal
	return m, modal.form.Init()
}

func (m dashboard) openIDokladModal() (tea.Model, tea.Cmd) {
	lastNumber, err := idoklad.LastDocumentNumber(m.root, m.cfg.Profile.VATID)
	if err != nil {
		m.notice = "Synchronizaci iDokladu nelze připravit: " + err.Error()
		return m, nil
	}
	title, description := idokladSyncPrompt(lastNumber)
	request := modalRequest{
		kind: modalIDoklad, title: title, description: description,
		affirmative: "Stáhnout novější", negative: "Zrušit",
	}
	m.modal = newConfirmModal(request, m.modalFormWidth(72))
	return m, m.modal.form.Init()
}

func idokladSyncPrompt(lastNumber string) (string, string) {
	browserInfo := "Po potvrzení se otevře samostatné okno Chromu. Pokud v něm ještě nejste přihlášen, přihlaste se do iDokladu; jakmile se zobrazí seznam vydaných faktur, dphcko automaticky pokračuje. "
	if lastNumber == "" {
		return "Zatím není evidován žádný stažený doklad.", browserInfo + "Potom stáhne všechny dostupné vydané faktury. Chcete pokračovat?"
	}
	return "Poslední stažený doklad: " + lastNumber, browserInfo + "Potom stáhne všechny doklady s novějším číslem než " + lastNumber + ". Chcete pokračovat?"
}

func (m dashboard) prepareGenerationAt(now time.Time) (tea.Model, tea.Cmd) {
	m.pending = nil
	m.generate = GenerateOptions{ConsumerDocuments: make(map[string]bool)}
	period := m.periods[m.selected]
	if period.Year == now.Year() && period.Month == now.Month() {
		m.pending = append(m.pending, modalRequest{
			kind:        modalCurrentPeriod,
			title:       "Zdaňovací období " + period.String() + " ještě není ukončené.",
			description: "Faktury přidané později nebudou v tomto výstupu zahrnuté. Opravdu chcete generovat už nyní?",
			affirmative: "Ano, generovat", negative: "Zrušit",
		})
	}
	for _, result := range m.results {
		if result.Err == nil {
			continue
		}
		if result.Invoice != nil && result.Invoice.Total > invoice.A4Threshold && result.Invoice.RecipientVATID == "" {
			m.pending = append(m.pending, modalRequest{
				kind:        modalConsumer,
				title:       fmt.Sprintf("%s je nad 10 000 Kč a nemá DIČ odběratele.", filepath.Base(result.Path)),
				description: "Je odběratel koncový spotřebitel? Potvrzením bude doklad zařazen do oddílu A.5.",
				affirmative: "Ano, zařadit do A.5", negative: "Ne, zastavit", path: result.Path,
			})
			continue
		}
		m.notice = fmt.Sprintf("Generování zastaveno: %s: %v", filepath.Base(result.Path), result.Err)
		return m, nil
	}
	existing, err := epo.ExistingPeriodOutputs(period.Dir, period.Year, int(period.Month))
	if err != nil {
		m.notice = "Generování nelze připravit: " + err.Error()
		return m, nil
	}
	if len(existing) > 0 {
		names := make([]string, len(existing))
		for i, path := range existing {
			names[i] = filepath.Base(path)
		}
		m.pending = append(m.pending, modalRequest{
			kind:        modalOverwrite,
			title:       "Výstupy pro " + period.String() + " už existují.",
			description: strings.Join(names, ", ") + "\n\nChcete je přepsat novými soubory?",
			affirmative: "Ano, přepsat", negative: "Ne, ponechat",
		})
	}
	return m.openNextGenerationModal()
}

func (m dashboard) openNextGenerationModal() (tea.Model, tea.Cmd) {
	if len(m.pending) == 0 {
		m.action = ActionGenerate
		return m, tea.Quit
	}
	request := m.pending[0]
	m.pending = m.pending[1:]
	m.modal = newConfirmModal(request, m.modalFormWidth(72))
	return m, m.modal.form.Init()
}

func newConfirmModal(request modalRequest, width int) *dashboardModal {
	modal := &dashboardModal{kind: request.kind, path: request.path, help: "←/→ vybrat · Enter potvrdit · Esc zavřít"}
	modal.form = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(request.title).
			Description(request.description).
			Affirmative(request.affirmative).
			Negative(request.negative).
			Value(&modal.confirmed),
	)).WithWidth(width).WithShowHelp(false)
	return modal
}

func (m dashboard) modalFormWidth(maxWidth int) int {
	if m.width <= 0 {
		return maxWidth
	}
	return max(28, min(maxWidth, m.width-10))
}

func (m dashboard) updateModal(message tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := message.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if key, ok := message.(tea.KeyPressMsg); ok && key.String() == "esc" {
		m.modal = nil
		m.pending = nil
		m.generate = GenerateOptions{}
		return m, nil
	}
	updated, cmd := m.modal.form.Update(message)
	m.modal.form = updated.(*huh.Form)
	switch m.modal.form.State {
	case huh.StateAborted:
		m.modal = nil
		m.pending = nil
		m.generate = GenerateOptions{}
		return m, cmd
	case huh.StateCompleted:
		next, nextCmd := m.completeModal()
		return next, tea.Batch(cmd, nextCmd)
	default:
		return m, cmd
	}
}

func (m dashboard) completeModal() (tea.Model, tea.Cmd) {
	modal := m.modal
	m.modal = nil
	switch modal.kind {
	case modalNewPeriod:
		year, _ := strconv.Atoi(modal.yearValue)
		monthNumber, _ := strconv.Atoi(modal.monthValue)
		dir := invoice.PeriodDirectory(m.root, year, time.Month(monthNumber))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			m.notice = "Období se nepodařilo vytvořit: " + err.Error()
			return m, nil
		}
		m.periods = DiscoverPeriods(m.root)
		for i, period := range m.periods {
			if period.Year == year && period.Month == time.Month(monthNumber) {
				m.selected = i
				break
			}
		}
		m.reload()
		m.notice = fmt.Sprintf("Založena složka %04d/%02d. Vložte do ní PDF faktury.", year, monthNumber)
		return m, nil
	case modalIDoklad:
		if !modal.confirmed {
			return m, nil
		}
		m.action = ActionImportIDoklad
		return m, tea.Quit
	case modalCurrentPeriod:
		if !modal.confirmed {
			return m.cancelGeneration()
		}
		return m.openNextGenerationModal()
	case modalConsumer:
		if !modal.confirmed {
			return m.cancelGeneration()
		}
		m.generate.ConsumerDocuments[modal.path] = true
		return m.openNextGenerationModal()
	case modalOverwrite:
		if !modal.confirmed {
			return m.cancelGeneration()
		}
		m.generate.Overwrite = true
		return m.openNextGenerationModal()
	default:
		return m, nil
	}
}

func (m dashboard) cancelGeneration() (tea.Model, tea.Cmd) {
	m.pending = nil
	m.generate = GenerateOptions{}
	m.notice = "Generování bylo zrušeno; žádné soubory se nezměnily."
	return m, nil
}

func (m dashboard) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render("DPHČKO")
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#35BB78"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E85D75"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5C451"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s %s · DIČ %s\n", title, m.cfg.Profile.FirstName, m.cfg.Profile.LastName, m.cfg.Profile.VATID)
	statusLine := ""
	if m.notice != "" {
		statusLine = strings.Join(strings.Fields(m.notice), " ")
		if m.width > 0 {
			statusLine = ansi.Truncate(statusLine, m.width, "…")
		}
		statusLine = okStyle.Render(statusLine)
	}
	b.WriteString(statusLine + "\n")
	b.WriteString("Zdaňovací období\n")
	if len(m.periods) == 0 {
		b.WriteString(muted.Render("  Zatím žádné. Stiskněte n pro založení minulého měsíce.") + "\n")
	} else {
		for i, period := range m.periods {
			prefix := " "
			status := "  "
			suffix := ""
			label := period.String()
			if period.HasEPOOutput {
				status = okStyle.Render("✓") + " "
			}
			if i == m.selected {
				label = selectedStyle.Render(label)
				suffix = " " + selectedStyle.Render("<")
			}
			fmt.Fprintf(&b, "%s%s%s%s\n", prefix, status, label, suffix)
		}
	}
	b.WriteString("\nFaktury ve vybraném období\n")
	if len(m.results) == 0 {
		b.WriteString(muted.Render(" Ve složce nejsou žádná PDF.") + "\n")
	}
	for _, result := range m.results {
		name := filepath.Base(result.Path)
		if result.Err != nil {
			fmt.Fprintf(&b, " %s %s — %v\n", errStyle.Render("✗"), name, result.Err)
		} else {
			fmt.Fprintf(&b, " %s %s — %s, %s\n", okStyle.Render("✓"), name, result.Invoice.Number, formatCZK(result.Invoice.Total))
		}
	}
	b.WriteString("\nSouhrn DPH z platných faktur\n")
	summary, invalid, summaryErr := summarizeResults(m.results)
	if summaryErr != nil {
		fmt.Fprintf(&b, "  %s %v\n", errStyle.Render("✗"), summaryErr)
	} else {
		if invalid > 0 {
			fmt.Fprintf(&b, "  %s Generování je zablokované. Chybné faktury: %d; součty zahrnují jen platné doklady.\n", errStyle.Render("!"), invalid)
		}
		fmt.Fprintf(&b, "  Platné doklady: %d · A.4: %d · A.5: %d\n", len(summary.Invoices), len(summary.A4), len(summary.A5))
		b.WriteString(muted.Render("  Oddíl KH          Základ 21 %        DPH 21 %") + "\n")
		fmt.Fprintf(&b, "  %-12s %14s  %14s\n", fmt.Sprintf("A.4 (%d)", len(summary.A4)), formatCZK(summary.Base-summary.A5Base), formatCZK(summary.Tax-summary.A5Tax))
		fmt.Fprintf(&b, "  %-12s %14s  %14s\n", fmt.Sprintf("A.5 (%d)", len(summary.A5)), formatCZK(summary.A5Base), formatCZK(summary.A5Tax))
		fmt.Fprintf(&b, "  %s\n", selectedStyle.Render(fmt.Sprintf("%-12s %14s  %14s", "Celkem", formatCZK(summary.Base), formatCZK(summary.Tax))))
		fmt.Fprintf(&b, "  Celkem včetně DPH: %s\n", formatCZK(summary.Total))
	}
	b.WriteString("\n")
	b.WriteString(renderDashboardMenu(m.help) + "\n")
	content := b.String()
	if m.modal != nil {
		content = m.renderModal(content)
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "DPHČKO"
	return view
}

func (m dashboard) renderModal(background string) string {
	modalBody := m.modal.form.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(m.modal.help)
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Background(lipgloss.Color("#202124")).
		Padding(1, 2).
		Render(modalBody)
	viewportWidth := m.width
	if viewportWidth <= 0 {
		viewportWidth = max(lipgloss.Width(background), lipgloss.Width(modal)+4)
	}
	viewportHeight := m.height
	if viewportHeight <= 0 {
		viewportHeight = max(lipgloss.Height(background), lipgloss.Height(modal)+4)
	}
	width := max(viewportWidth, lipgloss.Width(background), lipgloss.Width(modal)+4)
	height := max(viewportHeight, lipgloss.Height(background), lipgloss.Height(modal)+4)
	base := lipgloss.NewStyle().Width(width).Height(height).Render(background)
	x := max(0, (viewportWidth-lipgloss.Width(modal))/2)
	y := max(1, (viewportHeight-lipgloss.Height(modal))/3)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base).Z(0),
		lipgloss.NewLayer(modal).X(x).Y(y).Z(1),
	).Render()
}

func renderDashboardMenu(help bool) string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#102018")).Background(lipgloss.Color("#5AF78E")).Padding(0, 1)
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("#B8B8B8"))
	separator := "   "

	item := func(key, description string, keyStyle lipgloss.Style) string {
		return keyStyle.Render(key) + " " + label.Render(description)
	}
	helpLabel := "nápověda"
	if help {
		helpLabel = "skrýt nápovědu"
	}
	items := []string{
		item("N", "nové období", keyStyle),
		item("G", "generovat", keyStyle),
		item("O", "otevřít EPO", keyStyle),
		item("I", "synchronizovat iDoklad", keyStyle),
		item("R", "načíst", keyStyle),
		item("C", "profil", keyStyle),
		item("?", helpLabel, keyStyle),
		item("Q", "konec", keyStyle),
	}
	if help {
		items = append(items, label.Render("↑/↓ nebo J/K vybrat období"))
	}
	return strings.Join(items, separator)
}

func summarizeResults(results []invoice.FileResult) (tax.Summary, int, error) {
	invoices := make([]invoice.Invoice, 0, len(results))
	invalid := 0
	for _, result := range results {
		if result.Err != nil || result.Invoice == nil {
			invalid++
			continue
		}
		invoices = append(invoices, *result.Invoice)
	}
	summary, err := tax.Build(invoices)
	return summary, invalid, err
}

func formatCZK(value invoice.Money) string {
	amount := int64(value)
	sign := ""
	var magnitude uint64
	if amount < 0 {
		sign = "-"
		magnitude = uint64(-(amount + 1)) + 1
	} else {
		magnitude = uint64(amount)
	}
	koruny := strconv.FormatUint(magnitude/100, 10)
	for i := len(koruny) - 3; i > 0; i -= 3 {
		koruny = koruny[:i] + " " + koruny[i:]
	}
	return fmt.Sprintf("%s%s,%02d Kč", sign, koruny, magnitude%100)
}

func (m *dashboard) reload() {
	if len(m.periods) == 0 {
		m.results = nil
		return
	}
	period := m.periods[m.selected]
	m.results = invoice.ScanPeriod(period.Dir, invoice.ValidationOptions{
		IssuerVATID: m.cfg.Profile.VATID, Year: period.Year, Month: period.Month,
	})
}

func DiscoverPeriods(root string) []Period {
	var periods []Period
	years, _ := os.ReadDir(root)
	for _, yearEntry := range years {
		if !yearEntry.IsDir() || len(yearEntry.Name()) != 4 {
			continue
		}
		year, err := strconv.Atoi(yearEntry.Name())
		if err != nil {
			continue
		}
		months, _ := os.ReadDir(filepath.Join(root, yearEntry.Name()))
		for _, monthEntry := range months {
			month, err := strconv.Atoi(monthEntry.Name())
			if !monthEntry.IsDir() || err != nil || month < 1 || month > 12 {
				continue
			}
			dir := filepath.Join(root, yearEntry.Name(), monthEntry.Name())
			periods = append(periods, Period{
				Year: year, Month: time.Month(month), Dir: dir,
				HasEPOOutput: periodHasEPOOutput(dir, year, month),
			})
		}
	}
	sort.Slice(periods, func(i, j int) bool {
		if periods[i].Year != periods[j].Year {
			return periods[i].Year < periods[j].Year
		}
		return periods[i].Month < periods[j].Month
	})
	return periods
}

func periodHasEPOOutput(periodDir string, year, month int) bool {
	path := filepath.Join(periodDir, "vystup", fmt.Sprintf("DPHDP3_%04d-%02d.xml", year, month))
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}
