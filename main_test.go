package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TESTING DE MODEL
func TestModelShouldQuitWhenQKeyPressed(t *testing.T) {
	// Arrange
	m := model{width: 80, height: 24, ready: true}
	// Act
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	// Assert
	if cmd == nil {
		t.Fatal("esperaba un Cmd de quit, obtuve nil")
	}
	um := updated.(model)
	if um.width != 80 {
		t.Errorf("width no deberia cambiar: esperaba 80, obtuve %d", um.width)
	}
}

func TestModelShouldUpdateDimensionsWhenWindowResized(t *testing.T) {
	// Arrange
	tests := []struct {
		name           string
		width          int
		height         int
		expectedReady  bool
	}{
		{"resize a 120x40", 120, 40, true},
		{"resize a 80x24", 80, 24, true},
	}
	// Act
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{}
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			um := updated.(model)
			// Assert
			if um.width != tt.width || um.height != tt.height {
				t.Errorf("tamaño esperado %dx%d, obtuve %dx%d", tt.width, tt.height, um.width, um.height)
			}
			if um.ready != tt.expectedReady {
				t.Errorf("ready esperado %v, obtuve %v", tt.expectedReady, um.ready)
			}
		})
	}
}

func TestModelShouldIncrementElapsedWhenTickReceived(t *testing.T) {
	// Arrange
	tests := []struct {
		name            string
		elapsedInicial  int
		elapsedEsperado int
	}{
		{"desde cero", 0, 1},
		{"acumulado", 5, 6},
	}
	// Act
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{width: 80, height: 24, ready: true, elapsed: tt.elapsedInicial}
			updated, cmd := m.Update(tickMsg(time.Now()))
			um := updated.(model)
			// Assert
			if um.elapsed != tt.elapsedEsperado {
				t.Errorf("elapsed esperado %d, obtuve %d", tt.elapsedEsperado, um.elapsed)
			}
			if cmd == nil {
				t.Error("despues de un tick, se debe re-encolar el proximo tickCmd")
			}
		})
	}
}

func TestInitShouldReturnTickCmdWhenCalled(t *testing.T) {
	// Arrange
	m := model{}
	// Act
	cmd := m.Init()
	// Assert
	if cmd == nil {
		t.Error("Init deberia retornar tickCmd para iniciar el loop de animacion")
	}
}

// TESTING DE VISTA
func TestViewShouldShowVersionWhenReady(t *testing.T) {
	// Arrange
	m := model{width: 80, height: 24, ready: true}
	// Act
	view := m.View()
	// Assert
	if !strings.Contains(view, version) {
		t.Errorf("la vista deberia contener la version %q", version)
	}
}

func TestViewShouldShowLoadingWhenNotReady(t *testing.T) {
	// Arrange
	m := model{ready: false}
	// Act
	view := m.View()
	// Assert
	if !strings.Contains(view, "Cargando") {
		t.Error("antes de recibir WindowSizeMsg, deberia mostrar 'Cargando...'")
	}
}

func TestViewShouldShowCounterWhenReady(t *testing.T) {
	// Arrange
	m := model{width: 80, height: 24, ready: true, elapsed: 42}
	// Act
	view := m.View()
	// Assert
	if !strings.Contains(view, "42s") {
		t.Error("la vista deberia mostrar el contador de segundos transcurridos")
	}
}
