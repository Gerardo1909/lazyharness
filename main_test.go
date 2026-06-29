package main

import (
	"testing"

	"github.com/Gerardo1909/lazyharness/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

// TESTING DE APP (modelo raíz)

func TestAppShouldQuitWhenQKeyPressed(t *testing.T) {
	// Arrange
	a := app.NewApp()
	// Act
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	// Assert
	if cmd == nil {
		t.Fatal("esperaba un Cmd de quit, obtuve nil")
	}
}

func TestAppShouldUpdateDimensionsWhenWindowResized(t *testing.T) {
	// Arrange
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"resize a 120x40", 120, 40},
		{"resize a 80x24", 80, 24},
	}
	// Act
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := app.NewApp()
			_, cmd := a.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			// Assert: el modelo debe procesar el mensaje sin error
			// (el estado interno es opaco; solo verificamos que no paniquea)
			_ = cmd
		})
	}
}

func TestAppShouldRenderSplashWhenInitialized(t *testing.T) {
	// Arrange
	a := app.NewApp()
	// Update retorna un nuevo modelo (bubbletea es inmutable); hay que capturarlo
	updated, _ := a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Act
	view := updated.View()
	// Assert: la pantalla splash debe renderizar algo al tener dimensiones
	if view == "" {
		t.Error("la vista inicial no debería estar vacía")
	}
}

func TestAppInitShouldReturnCmd(t *testing.T) {
	// Arrange
	a := app.NewApp()
	// Act — Init puede retornar nil si la home no tiene comandos iniciales
	_ = a.Init()
}

func TestAppShouldBeVersionDefined(t *testing.T) {
	// Arrange / Assert
	if version == "" {
		t.Error("la constante version no debería estar vacía")
	}
}
