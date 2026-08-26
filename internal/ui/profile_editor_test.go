package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"dphcko/internal/config"
)

func editorTestConfig() config.Config {
	return config.Config{Format: config.CurrentFormat, Profile: config.Profile{
		ICO: "12345678", VATID: "CZ12345678", FirstName: "Jan", LastName: "Novák",
		Street: "Dlouhá", HouseNumber: "1", OrientationNo: "2a", City: "Praha",
		PostalCode: "11000", Country: "CZ", TaxOffice: "451", TaxOfficeBranch: "2001",
		NACE: "62010", Phone: "+420123456789", Email: "jan@example.cz",
	}}
}

func TestProfileEditorShowsCompleteProfileAtOnce(t *testing.T) {
	m := newProfileEditor(t.TempDir(), editorTestConfig())
	view := m.View().Content
	for _, field := range profileFields {
		if !strings.Contains(view, field.label) {
			t.Errorf("editor nezobrazuje pole %q", field.label)
		}
	}
	for _, value := range []string{"12345678", "CZ12345678", "Jan", "Novák", "Dlouhá", "Praha", "jan@example.cz"} {
		if !strings.Contains(view, value) {
			t.Errorf("editor nezobrazuje hodnotu %q", value)
		}
	}
}

func TestProfileEditorNavigationAndCancel(t *testing.T) {
	m := newProfileEditor(t.TempDir(), editorTestConfig())
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = next.(profileEditor)
	if m.selected != 1 || !m.inputs[1].Focused() || m.inputs[0].Focused() {
		t.Fatalf("šipka dolů nevybrala druhé pole: selected=%d", m.selected)
	}

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = next.(profileEditor)
	if !m.canceled || cmd == nil {
		t.Fatal("Esc musí editor ukončit bez uložení")
	}
}

func TestProfileEditorHighlightsChangedField(t *testing.T) {
	m := newProfileEditor(t.TempDir(), editorTestConfig())
	m.inputs[2].SetValue("Jana")
	m.applyInputStyles()
	if m.changedCount() != 1 || !m.fieldChanged(2) {
		t.Fatalf("počet změn = %d, chci 1", m.changedCount())
	}
	view := m.View().Content
	if !strings.Contains(view, "Neuložené změny: 1") || !strings.Contains(view, "● změněno") {
		t.Fatal("změněné pole není v editoru viditelně označeno")
	}
}

func TestProfileEditorEnterSavesChanges(t *testing.T) {
	dir := t.TempDir()
	m := newProfileEditor(dir, editorTestConfig())
	m.inputs[2].SetValue("Jana")

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = next.(profileEditor)
	if cmd == nil || !m.saving {
		t.Fatal("Enter musí zahájit uložení")
	}
	next, _ = m.Update(cmd())
	m = next.(profileEditor)
	if !m.saved {
		t.Fatal("profil nebyl označen jako uložený")
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Profile.FirstName != "Jana" {
		t.Fatalf("uložené jméno = %q, chci Jana", loaded.Profile.FirstName)
	}
}

func TestProfileEditorRejectsInvalidChange(t *testing.T) {
	m := newProfileEditor(t.TempDir(), editorTestConfig())
	m.inputs[0].SetValue("12")
	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = next.(profileEditor)
	if cmd != nil || m.err == nil || m.saving {
		t.Fatal("neplatný profil se nesmí začít ukládat")
	}
	if !strings.Contains(m.View().Content, "Nelze uložit") {
		t.Fatal("editor nezobrazuje chybu validace")
	}
}
