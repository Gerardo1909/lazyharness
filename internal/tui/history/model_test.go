package history

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Gerardo1909/lazyharness/internal/domain"
	"github.com/Gerardo1909/lazyharness/internal/storage"
	"github.com/Gerardo1909/lazyharness/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func makeTestHarnessWithHistory(t *testing.T) domain.Harness {
	t.Helper()
	dir := t.TempDir()
	h, _ := domain.NewHarness("dev-flow", dir, "xml")
	_ = h.AddRole(domain.Role{Name: "reviewer", Color: "#e0af68", PromptFile: "reviewer.xml"})
	_ = storage.SaveHarness(dir, h)
	// create prompt file and commit multiple times so history tests have data
	for _, msg := range []string{"v1", "v2", "v3"} {
		_ = storage.WritePromptFile(dir, "reviewer.xml", "contenido "+msg)
		_ = storage.Commit(dir, msg)
	}
	h.ProjectDir = dir
	return h
}

func makeTestHarnessEmpty() domain.Harness {
	h, _ := domain.NewHarness("dev-flow", os.TempDir(), "xml")
	_ = h.AddRole(domain.Role{Name: "reviewer", Color: "#e0af68", PromptFile: "reviewer.xml"})
	return h
}

func TestHistoryShouldEmitGoBackWhenEscPressed(t *testing.T) {
	m := New(makeTestHarnessEmpty(), "reviewer", 120, 40)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esperaba Cmd de GoBack, obtuve nil")
	}
	msg := cmd()
	if _, ok := msg.(tui.GoBackMsg); !ok {
		t.Errorf("esperaba GoBackMsg, obtuve %T", msg)
	}
}

func TestHistoryShouldNavigateCommitsWhenJKPressed(t *testing.T) {
	if _, err := filepath.Abs("git"); err != nil {
		t.Skip("git no disponible")
	}
	m := New(makeTestHarnessWithHistory(t), "reviewer", 120, 40)
	if len(m.commits) < 2 {
		t.Fatalf("se esperaban al menos 2 commits, obtuve %d", len(m.commits))
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if updated.selectedIdx != 1 {
		t.Errorf("esperaba índice 1 después de j, obtuve %d", updated.selectedIdx)
	}
	updated2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if updated2.selectedIdx != 0 {
		t.Errorf("esperaba índice 0 después de k, obtuve %d", updated2.selectedIdx)
	}
}

func TestHistoryShouldTransitionToConfirmWhenRPressed(t *testing.T) {
	if _, err := filepath.Abs("git"); err != nil {
		t.Skip("git no disponible")
	}
	m := New(makeTestHarnessWithHistory(t), "reviewer", 120, 40)
	if len(m.commits) == 0 {
		t.Skip("sin commits para confirmar")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if updated.state != stateConfirm {
		t.Errorf("R debería pasar a stateConfirm, obtuve %d", updated.state)
	}
}

func TestHistoryShouldCancelConfirmWhenEscPressedInConfirmState(t *testing.T) {
	m := New(makeTestHarnessEmpty(), "reviewer", 120, 40)
	m.state = stateConfirm
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.state != stateBrowsing {
		t.Errorf("esc en confirm debería volver a stateBrowsing, obtuve %d", updated.state)
	}
}

func TestHistoryShouldRenderNonEmptyView(t *testing.T) {
	m := New(makeTestHarnessEmpty(), "reviewer", 120, 40)
	view := m.View()
	if view == "" {
		t.Error("la vista del historial no debería estar vacía")
	}
}
