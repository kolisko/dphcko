package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"dphcko/internal/config"
)

const profileLabelWidth = 27

type profileField struct {
	label string
	get   func(config.Profile) string
	set   func(*config.Profile, string)
	limit int
}

var profileFields = []profileField{
	{label: "IČO", get: func(p config.Profile) string { return p.ICO }, set: func(p *config.Profile, v string) { p.ICO = v }, limit: 8},
	{label: "DIČ", get: func(p config.Profile) string { return p.VATID }, set: func(p *config.Profile, v string) { p.VATID = v }, limit: 12},
	{label: "Jméno", get: func(p config.Profile) string { return p.FirstName }, set: func(p *config.Profile, v string) { p.FirstName = v }, limit: 20},
	{label: "Příjmení", get: func(p config.Profile) string { return p.LastName }, set: func(p *config.Profile, v string) { p.LastName = v }, limit: 36},
	{label: "Ulice nebo část obce", get: func(p config.Profile) string { return p.Street }, set: func(p *config.Profile, v string) { p.Street = v }, limit: 38},
	{label: "Číslo popisné", get: func(p config.Profile) string { return p.HouseNumber }, set: func(p *config.Profile, v string) { p.HouseNumber = v }, limit: 6},
	{label: "Číslo orientační", get: func(p config.Profile) string { return p.OrientationNo }, set: func(p *config.Profile, v string) { p.OrientationNo = v }, limit: 4},
	{label: "Obec", get: func(p config.Profile) string { return p.City }, set: func(p *config.Profile, v string) { p.City = v }, limit: 48},
	{label: "PSČ", get: func(p config.Profile) string { return p.PostalCode }, set: func(p *config.Profile, v string) { p.PostalCode = v }, limit: 6},
	{label: "Stát", get: func(p config.Profile) string { return p.Country }, set: func(p *config.Profile, v string) { p.Country = v }, limit: 2},
	{label: "Kód finančního úřadu", get: func(p config.Profile) string { return p.TaxOffice }, set: func(p *config.Profile, v string) { p.TaxOffice = v }, limit: 3},
	{label: "Kód územního pracoviště", get: func(p config.Profile) string { return p.TaxOfficeBranch }, set: func(p *config.Profile, v string) { p.TaxOfficeBranch = v }, limit: 4},
	{label: "CZ-NACE", get: func(p config.Profile) string { return p.NACE }, set: func(p *config.Profile, v string) { p.NACE = v }, limit: 6},
	{label: "Telefon", get: func(p config.Profile) string { return p.Phone }, set: func(p *config.Profile, v string) { p.Phone = v }, limit: 14},
	{label: "E-mail", get: func(p config.Profile) string { return p.Email }, set: func(p *config.Profile, v string) { p.Email = v }, limit: 255},
}

type profileSavedMsg struct {
	cfg config.Config
	err error
}

type profileEditor struct {
	root     string
	original config.Config
	cfg      config.Config
	inputs   []textinput.Model
	selected int
	width    int
	saving   bool
	saved    bool
	canceled bool
	err      error
}

// RunProfileEditor opens the regular profile editor. The first-run wizard is
// intentionally separate: editing an existing profile never contacts ARES.
func RunProfileEditor(root string, cfg config.Config) (config.Config, bool, error) {
	program := tea.NewProgram(newProfileEditor(root, cfg))
	final, err := program.Run()
	if err != nil {
		return cfg, false, err
	}
	result := final.(profileEditor)
	if result.saved {
		return result.cfg, true, nil
	}
	return cfg, false, nil
}

func newProfileEditor(root string, cfg config.Config) profileEditor {
	m := profileEditor{root: root, original: cfg, cfg: cfg, width: 100}
	m.inputs = make([]textinput.Model, len(profileFields))
	for i, field := range profileFields {
		input := textinput.New()
		input.Prompt = ""
		input.Placeholder = "(prázdné)"
		input.CharLimit = field.limit
		input.SetValue(field.get(cfg.Profile))
		input.SetWidth(48)
		m.inputs[i] = input
	}
	m.inputs[0].Focus()
	m.applyInputStyles()
	return m
}

func (m profileEditor) Init() tea.Cmd {
	return textinput.Blink
}

func (m profileEditor) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case profileSavedMsg:
		return m.handleSaved(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.resizeInputs()
	case tea.KeyPressMsg:
		if m.saving {
			return m, nil
		}
		switch msg.String() {
		case "esc", "ctrl+c":
			m.canceled = true
			return m, tea.Quit
		case "up", "shift+tab":
			return m.move(-1)
		case "down", "tab":
			return m.move(1)
		case "enter", "ctrl+s", "S":
			cfg := m.configFromInputs()
			if err := cfg.Validate(); err != nil {
				m.err = err
				m.applyInputStyles()
				return m, nil
			}
			m.saving = true
			m.err = nil
			return m, saveProfileCmd(m.root, cfg)
		}
	}

	var cmd tea.Cmd
	m.inputs[m.selected], cmd = m.inputs[m.selected].Update(message)
	m.err = nil
	m.applyInputStyles()
	return m, cmd
}

func (m profileEditor) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	active := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5C451"))
	changed := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E879F9"))
	errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E85D75"))

	var b strings.Builder
	b.WriteString(title.Render("PROFIL PLÁTCE"))
	b.WriteString("  ")
	b.WriteString(muted.Render(config.FileName))
	b.WriteString("\n")
	if count := m.changedCount(); count > 0 {
		fmt.Fprintf(&b, "%s\n\n", changed.Render(fmt.Sprintf("● Neuložené změny: %d", count)))
	} else {
		b.WriteString(muted.Render("Beze změn") + "\n\n")
	}

	for i, field := range profileFields {
		prefix := "  "
		label := lipgloss.NewStyle().Foreground(lipgloss.Color("#B8B8B8")).Render(fmt.Sprintf("%-*s", profileLabelWidth, field.label))
		if i == m.selected {
			prefix = active.Render("› ")
			label = active.Render(fmt.Sprintf("%-*s", profileLabelWidth, field.label))
		}
		marker := ""
		if m.fieldChanged(i) {
			marker = changed.Render(" ● změněno")
		}
		fmt.Fprintf(&b, "%s%s %s%s\n", prefix, label, m.inputs[i].View(), marker)
	}

	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(errStyle.Render("Nelze uložit: "+m.err.Error()) + "\n")
	} else if m.saving {
		b.WriteString(changed.Render("Ukládám profil…") + "\n")
	}
	b.WriteString(muted.Render("↑/↓ nebo Tab pole · ←/→ kurzor · Enter / Ctrl+S / S uložit · Esc odejít bez uložení") + "\n")

	view := tea.NewView(b.String())
	view.AltScreen = true
	view.WindowTitle = "DPHČKO — profil plátce"
	return view
}

func (m profileEditor) move(delta int) (tea.Model, tea.Cmd) {
	m.inputs[m.selected].Blur()
	m.selected = (m.selected + delta + len(m.inputs)) % len(m.inputs)
	cmd := m.inputs[m.selected].Focus()
	m.inputs[m.selected].CursorEnd()
	m.applyInputStyles()
	return m, cmd
}

func (m *profileEditor) resizeInputs() {
	width := m.width - profileLabelWidth - 13
	if width < 16 {
		width = 16
	}
	if width > 60 {
		width = 60
	}
	for i := range m.inputs {
		m.inputs[i].SetWidth(width)
	}
}

func (m *profileEditor) applyInputStyles() {
	for i := range m.inputs {
		styles := textinput.DefaultDarkStyles()
		if m.fieldChanged(i) {
			styles.Focused.Text = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E879F9"))
			styles.Blurred.Text = styles.Focused.Text
			styles.Cursor.Color = lipgloss.Color("#E879F9")
		} else {
			styles.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
			styles.Blurred.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#D0D0D0"))
			styles.Cursor.Color = lipgloss.Color("#F5C451")
		}
		m.inputs[i].SetStyles(styles)
	}
}

func (m profileEditor) configFromInputs() config.Config {
	cfg := m.cfg
	for i, field := range profileFields {
		field.set(&cfg.Profile, strings.TrimSpace(m.inputs[i].Value()))
	}
	cfg.Profile.VATID = strings.ToUpper(strings.ReplaceAll(cfg.Profile.VATID, " ", ""))
	cfg.Profile.PostalCode = strings.ReplaceAll(cfg.Profile.PostalCode, " ", "")
	cfg.Profile.Country = strings.ToUpper(cfg.Profile.Country)
	return cfg
}

func (m profileEditor) fieldChanged(index int) bool {
	return m.inputs[index].Value() != profileFields[index].get(m.original.Profile)
}

func (m profileEditor) changedCount() int {
	count := 0
	for i := range m.inputs {
		if m.fieldChanged(i) {
			count++
		}
	}
	return count
}

func saveProfileCmd(root string, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		err := config.Save(root, cfg)
		return profileSavedMsg{cfg: cfg, err: err}
	}
}

func (m profileEditor) handleSaved(msg profileSavedMsg) (tea.Model, tea.Cmd) {
	m.saving = false
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	m.cfg = msg.cfg
	m.saved = true
	return m, tea.Quit
}
