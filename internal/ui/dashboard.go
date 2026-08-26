package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"dphcko/internal/config"
	"dphcko/internal/invoice"
	"dphcko/internal/tax"
)

type Action int

const (
	ActionQuit Action = iota
	ActionNewPeriod
	ActionGenerate
	ActionConfig
	ActionOpenEPO
)

type Period struct {
	Year  int
	Month time.Month
	Dir   string
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
	notice   string
}

func RunDashboard(root string, cfg config.Config, notice string) (Action, *Period, error) {
	m := dashboard{root: root, cfg: cfg, periods: DiscoverPeriods(root), notice: notice}
	m.reload()
	program := tea.NewProgram(m)
	final, err := program.Run()
	if err != nil {
		return ActionQuit, nil, err
	}
	result := final.(dashboard)
	if len(result.periods) == 0 {
		return result.action, nil, nil
	}
	period := result.periods[result.selected]
	return result.action, &period, nil
}

func (m dashboard) Init() tea.Cmd { return nil }

func (m dashboard) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.action = ActionQuit
			return m, tea.Quit
		case "n":
			m.action = ActionNewPeriod
			return m, tea.Quit
		case "c":
			m.action = ActionConfig
			return m, tea.Quit
		case "o":
			m.action = ActionOpenEPO
			return m, tea.Quit
		case "g", "enter":
			if len(m.periods) > 0 {
				m.action = ActionGenerate
				return m, tea.Quit
			}
		case "r":
			m.reload()
			m.notice = "Složka období byla znovu načtena."
		case "?":
			m.help = !m.help
		case "up", "k":
			if m.selected > 0 {
				m.selected--
				m.reload()
			}
		case "down", "j":
			if m.selected+1 < len(m.periods) {
				m.selected++
				m.reload()
			}
		}
	}
	return m, nil
}

func (m dashboard) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render("DPHČKO")
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#35BB78"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E85D75"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5C451"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s %s · DIČ %s\n\n", title, m.cfg.Profile.FirstName, m.cfg.Profile.LastName, m.cfg.Profile.VATID)
	if m.notice != "" {
		fmt.Fprintf(&b, "%s\n\n", okStyle.Render(m.notice))
	}
	b.WriteString("Zdaňovací období\n")
	if len(m.periods) == 0 {
		b.WriteString(muted.Render("  Zatím žádné. Stiskněte n pro založení minulého měsíce.") + "\n")
	} else {
		for i, period := range m.periods {
			prefix := "  "
			label := period.String()
			if i == m.selected {
				prefix = "› "
				label = selectedStyle.Render(label)
			}
			fmt.Fprintf(&b, "%s%s\n", prefix, label)
		}
	}
	b.WriteString("\nFaktury ve vybraném období\n")
	if len(m.results) == 0 {
		b.WriteString(muted.Render("  Ve složce nejsou žádná PDF.") + "\n")
	}
	for _, result := range m.results {
		name := filepath.Base(result.Path)
		if result.Err != nil {
			fmt.Fprintf(&b, "  %s %s — %v\n", errStyle.Render("✗"), name, result.Err)
		} else {
			fmt.Fprintf(&b, "  %s %s — %s, %s\n", okStyle.Render("✓"), name, result.Invoice.Number, formatCZK(result.Invoice.Total))
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
	if m.help {
		b.WriteString("↑/↓ nebo j/k období · Enter/g generovat · o otevřít EPO · n nové období · r načíst · c profil · q konec · ? skrýt nápovědu\n")
	} else {
		b.WriteString(muted.Render("Enter/g generovat · o otevřít EPO · n nové · r načíst · c profil · ? nápověda · q konec") + "\n")
	}
	view := tea.NewView(b.String())
	view.AltScreen = true
	view.WindowTitle = "DPHČKO"
	return view
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
			periods = append(periods, Period{Year: year, Month: time.Month(month), Dir: filepath.Join(root, yearEntry.Name(), monthEntry.Name())})
		}
	}
	sort.Slice(periods, func(i, j int) bool {
		if periods[i].Year != periods[j].Year {
			return periods[i].Year > periods[j].Year
		}
		return periods[i].Month > periods[j].Month
	})
	return periods
}
