package ui

import (
	"strings"
	"testing"
)

func TestIDokladSyncPromptShowsLastDocumentNumber(t *testing.T) {
	title, description := idokladSyncPrompt("20260042")
	if title != "Poslední stažený doklad: 20260042" {
		t.Fatalf("neočekávaný titulek: %q", title)
	}
	for _, expected := range []string{"Chromu", "přihlaste se", "automaticky pokračuje", "20260042"} {
		if !strings.Contains(description, expected) {
			t.Fatalf("dialog neobsahuje %q: %q", expected, description)
		}
	}
}

func TestIDokladSyncPromptExplainsFirstSynchronization(t *testing.T) {
	title, description := idokladSyncPrompt("")
	if title == "" || description == "" {
		t.Fatalf("dialog první synchronizace není úplný: %q / %q", title, description)
	}
}
