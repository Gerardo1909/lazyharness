package editor

import (
	"testing"

	"github.com/Gerardo1909/lazyharness/internal/domain"
	"github.com/Gerardo1909/lazyharness/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func makeTestHarness() domain.Harness {
	h, _ := domain.NewHarness("dev-flow", "/tmp/test-proj", "xml")
	_ = h.AddRole(domain.Role{Name: "arquitecto", Color: "#f7768e", PromptFile: "arquitecto.xml"})
	return h
}

func TestEditorShouldEmitGoBackWhenEscPressed(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), "arquitecto", 120, 40)
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

func TestEditorShouldToggleAIWhenF2Pressed(t *testing.T) {
	m := New(makeTestHarness(), "arquitecto", 120, 40)
	if m.showAI {
		t.Fatal("la IA no debería estar activa al inicio")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	if !updated.showAI {
		t.Error("F2 debería activar la IA")
	}
	updated2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyF2})
	if updated2.showAI {
		t.Error("segundo F2 debería desactivar la IA")
	}
}

func TestEditorShouldTransitionToCommitStateWhenCtrlSPressed(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), "arquitecto", 120, 40)
	// Act
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	// Assert
	if updated.state != stateCommitMsg {
		t.Errorf("esperaba stateCommitMsg después de ctrl+s, obtuve %d", updated.state)
	}
}

func TestEditorShouldCancelCommitWhenEscPressedInCommitState(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), "arquitecto", 120, 40)
	m.state = stateCommitMsg
	// Act
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	// Assert
	if updated.state != stateEditing {
		t.Errorf("esc en commit state debería volver a stateEditing, obtuve %d", updated.state)
	}
}

func TestEditorShouldRenderNonEmptyView(t *testing.T) {
	// Arrange
	m := New(makeTestHarness(), "arquitecto", 120, 40)
	// Act
	view := m.View()
	// Assert
	if view == "" {
		t.Error("la vista del editor no debería estar vacía")
	}
}
