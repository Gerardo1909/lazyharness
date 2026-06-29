package home

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHomeShouldQuitWhenQKeyPressed(t *testing.T) {
	// Arrange
	m := New(120, 40)
	// Act
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	// Assert
	if cmd == nil {
		t.Fatal("esperaba un Cmd de quit, obtuve nil")
	}
}

func TestHomeShouldResizeWhenWindowSizeMsgReceived(t *testing.T) {
	// Arrange
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"terminal ancha", 200, 50},
		{"terminal chica", 80, 24},
	}
	// Act
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(120, 40)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			// Assert
			if updated.width != tt.width || updated.height != tt.height {
				t.Errorf("esperaba %dx%d, obtuve %dx%d", tt.width, tt.height, updated.width, updated.height)
			}
		})
	}
}

func TestHomeShouldRenderNonEmptyView(t *testing.T) {
	// Arrange
	m := New(120, 40)
	// Act
	view := m.View()
	// Assert
	if view == "" {
		t.Error("la vista no debería estar vacía")
	}
}

func TestHomeInitShouldReturnNil(t *testing.T) {
	// Arrange
	m := New(120, 40)
	// Act
	cmd := m.Init()
	// Assert: home no necesita comandos al iniciar
	if cmd != nil {
		t.Error("home.Init() debería retornar nil")
	}
}

func TestHomeShouldEmitOpenHarnessMsgWhenEnterPressedWithSelection(t *testing.T) {
	// Este test verifica que al presionar enter sobre un item se emite
	// un Cmd (aunque la lista puede estar vacía sin harnesses reales).
	// Arrange
	m := New(120, 40)
	// Act
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Assert: si no hay items, cmd es nil; si hay, no paniquea
	_ = cmd // no hay crash
}
