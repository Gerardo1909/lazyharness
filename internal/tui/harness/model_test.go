package harness

import (
	"testing"

	"github.com/Gerardo1909/lazyharness/internal/domain"
	"github.com/Gerardo1909/lazyharness/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func makeTestHarness() domain.Harness {
	h, _ := domain.NewHarness("dev-flow", "/tmp/test-proj", "xml")
	_ = h.AddRole(domain.Role{Name: "arquitecto", Color: "#f7768e", PromptFile: "arquitecto.xml"})
	_ = h.AddRole(domain.Role{Name: "reviewer", Color: "#e0af68", PromptFile: "reviewer.xml", Parent: "arquitecto"})
	h.Workflow = []string{"arquitecto", "reviewer"}
	return h
}

func TestHarnessShouldQuitWhenQKeyPressed(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), 120, 40)
	// Act
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	// Assert
	if cmd == nil {
		t.Fatal("esperaba Cmd de quit, obtuve nil")
	}
}

func TestHarnessShouldEmitGoBackWhenEscPressed(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), 120, 40)
	// Act
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	// Assert
	if cmd == nil {
		t.Fatal("esperaba Cmd de GoBack, obtuve nil")
	}
	msg := cmd()
	if _, ok := msg.(tui.GoBackMsg); !ok {
		t.Errorf("esperaba GoBackMsg, obtuve %T", msg)
	}
}

func TestHarnessShouldTogglePanelWhenTabPressed(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), 120, 40)
	panelInicial := m.active
	// Act
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Assert
	if updated.active == panelInicial {
		t.Error("tab debería cambiar el panel activo")
	}
}

func TestHarnessShouldNavigateRolesWhenJKPressed(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), 120, 40)
	if m.selectedIdx != 0 {
		t.Fatalf("índice inicial debería ser 0, obtuve %d", m.selectedIdx)
	}
	// Act: bajar
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	// Assert
	if updated.selectedIdx != 1 {
		t.Errorf("esperaba índice 1 después de j, obtuve %d", updated.selectedIdx)
	}
	// Act: subir
	updated2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if updated2.selectedIdx != 0 {
		t.Errorf("esperaba índice 0 después de k, obtuve %d", updated2.selectedIdx)
	}
}

func TestHarnessShouldEmitOpenEditorMsgWhenEPressed(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), 120, 40)
	// Act
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	// Assert
	if cmd == nil {
		t.Fatal("esperaba Cmd para abrir editor, obtuve nil")
	}
	msg := cmd()
	openMsg, ok := msg.(tui.OpenEditorMsg)
	if !ok {
		t.Fatalf("esperaba OpenEditorMsg, obtuve %T", msg)
	}
	if openMsg.RoleName != "arquitecto" {
		t.Errorf("esperaba rol arquitecto, obtuve %q", openMsg.RoleName)
	}
}

func TestHarnessShouldEmitOpenHistoryMsgWhenHPressed(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), 120, 40)
	// Act
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	// Assert
	if cmd == nil {
		t.Fatal("esperaba Cmd para abrir historial, obtuve nil")
	}
	msg := cmd()
	if _, ok := msg.(tui.OpenHistoryMsg); !ok {
		t.Errorf("esperaba OpenHistoryMsg, obtuve %T", msg)
	}
}

func TestHarnessShouldRenderNonEmptyView(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), 120, 40)
	// Act
	view := m.View()
	// Assert
	if view == "" {
		t.Error("la vista no debería estar vacía")
	}
}
